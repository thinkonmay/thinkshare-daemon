package turn

import (
	"flag"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/pion/stun/v2"
	"github.com/pion/turn/v3"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

const (
	realm = "thinkmay.net"
)

type TurnServer struct {
	s *turn.Server
}

func Open(username, password string, min_port, max_port, port int) (*TurnServer, error) {
	ip, err := system.GetPublicIPCurl()
	if err != nil {
		return nil, err
	}
	s, err := SetupTurn(
		username, password,
		ip,
		port,
		min_port,
		max_port)
	if err != nil {
		log.PushLog("failed to setup turn account: %s", err.Error())
		return nil, err
	}

	return &TurnServer{s}, nil
}

func (t *TurnServer) Close() {
	t.s.Close()
}

// stunLogger wraps a PacketConn and prints incoming/outgoing STUN packets
// This pattern could be used to capture/inspect/modify data as well
type stunLogger struct {
	net.PacketConn
}

func (s *stunLogger) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if n, err = s.PacketConn.WriteTo(p, addr); err == nil && stun.IsMessage(p) {
		msg := &stun.Message{Raw: p}
		if err = msg.Decode(); err != nil {
			return
		}

		// log.PushLog("[%s] Outbound STUN to %s: %s", time.Now().Format(time.RFC850), addr.String(), msg.String())
	}

	return
}

func (s *stunLogger) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if n, addr, err = s.PacketConn.ReadFrom(p); err == nil && stun.IsMessage(p) {
		msg := &stun.Message{Raw: p}
		if err = msg.Decode(); err != nil {
			return
		}

		// log.PushLog("[%s] Inbound  STUN to %s: %s", time.Now().Format(time.RFC850), addr.String(), msg.String())
	}

	return
}

func SetupTurn(
	username string,
	password string,
	publicip string,
	port int,
	min int,
	max int) (t *turn.Server, err error) {
	flag.Parse()

	// Create a UDP listener to pass into pion/turn
	// pion/turn itself doesn't allocate any UDP sockets, but lets the user pass them in
	// this allows us to add logging, storage or modify inbound/outbound traffic
	udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		return nil, fmt.Errorf("Failed to create TURN server listener: %s", err)
	}

	// Cache -users flag for easy lookup later
	// If passwords are stored they should be saved to your DB hashed using turn.GenerateAuthKey
	usersMap := map[string][]byte{}
	usersMap[username] = turn.GenerateAuthKey(username, realm, password)

	go func() {
		if err != nil {
			return
		}

		time.Sleep(time.Hour * 24)
		t.Close()
	}()

	return turn.NewServer(turn.ServerConfig{
		Realm: realm,
		// Set AuthHandler callback
		// This is called every time a user tries to authenticate with the TURN server
		// Return the key for that user, or false when no user is found
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			// log.PushLog("[%s] Incoming TURN: Request from %s", time.Now().Format(time.RFC850), srcAddr.String())
			if key, ok := usersMap[username]; ok {
				return key, true
			}
			return nil, false
		},
		// PacketConnConfigs is a list of UDP Listeners and the configuration around them
		PacketConnConfigs: []turn.PacketConnConfig{
			{
				PacketConn: &stunLogger{udpListener},
				RelayAddressGenerator: &turn.RelayAddressGeneratorPortRange{
					RelayAddress: net.ParseIP(publicip), // Claim that we are listening on IP passed by user (This should be your Public IP)
					Address:      "0.0.0.0",             // But actually be listening on every interface
					MinPort:      uint16(min),
					MaxPort:      uint16(max),
				},
			},
		},
	})
}
