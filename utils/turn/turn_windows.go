package turn

import (
	"fmt"
	"net"
	"strconv"
	"sync"

	"github.com/pion/turn/v4"
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

	udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		return nil, err
	}

	relayAddressGenerator, err := NewGenerator(min_port, max_port, ip)
	if err != nil {
		return nil, err
	}

	if ret.Server, err = turn.NewServer(turn.ServerConfig{
		Realm: realm,
		PacketConnConfigs: []turn.PacketConnConfig{
			turn.PacketConnConfig{
				PacketConn:            udpListener,
				RelayAddressGenerator: relayAddressGenerator,
			},
		},
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
