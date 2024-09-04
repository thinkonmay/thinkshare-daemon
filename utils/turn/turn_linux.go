package turn

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"syscall"

	"github.com/pion/turn/v4"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"golang.org/x/sys/unix"
)

const (
	realm     = "thinkmay.net"
	threadNum = 16
)

type TurnServer struct {
	*turn.Server
	mut      *sync.Mutex
	usersMap map[string][]byte
}

func NewServer(min_port, max_port, port int, ip string) (*TurnServer, error) {
	ret := &TurnServer{
		mut:      &sync.Mutex{},
		usersMap: map[string][]byte{},
	}

	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		return nil, err
	}

	// Create `numThreads` UDP listeners to pass into pion/turn
	// pion/turn itself doesn't allocate any UDP sockets, but lets the user pass them in
	// this allows us to add logging, storage or modify inbound/outbound traffic
	// UDP listeners share the same local address:port with setting SO_REUSEPORT and the kernel
	// will load-balance received packets per the IP 5-tuple
	listenerConfig := &net.ListenConfig{
		Control: func(network, address string, conn syscall.RawConn) error { // nolint: revive
			var operr error
			if err = conn.Control(func(fd uintptr) {
				operr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			}); err != nil {
				return err
			}

			return operr
		},
	}

	relayAddressGenerator, err := NewGenerator(min_port, max_port, ip)
	if err != nil {
		return nil, err
	}

	packetConnConfigs := make([]turn.PacketConnConfig, threadNum)
	for i := 0; i < threadNum; i++ {
		conn, listErr := listenerConfig.ListenPacket(context.Background(), addr.Network(), addr.String())
		if listErr != nil {
			log.PushLog("Failed to allocate UDP listener at %s:%s", addr.Network(), addr.String())
		}

		packetConnConfigs[i] = turn.PacketConnConfig{
			PacketConn:            conn,
			RelayAddressGenerator: relayAddressGenerator,
		}
	}

	if ret.Server, err = turn.NewServer(turn.ServerConfig{
		Realm:             realm,
		PacketConnConfigs: packetConnConfigs,
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			ret.mut.Lock()
			defer ret.mut.Unlock()
			if key, ok := ret.usersMap[username]; ok {
				return key, true
			} else {
				return nil, false
			}
		},
	}); err != nil {
		return nil, fmt.Errorf("Failed to create TURN server : %s", err)
	}

	return ret, nil
}

func (t *TurnServer) DeallocateUser(username string) {
	t.mut.Lock()
	defer t.mut.Unlock()

	delete(t.usersMap, username)
}
func (t *TurnServer) AllocateUser(username, password string) {
	t.mut.Lock()
	defer t.mut.Unlock()

	t.usersMap[username] = turn.GenerateAuthKey(username, realm, password)
}
func (t *TurnServer) Close() {
	t.Server.Close()
}
