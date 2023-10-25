package turn

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/pion/stun/v2"
	"github.com/pion/turn/v3"
	"github.com/thinkonmay/thinkshare-daemon/credential"
)


const (
	threadNum = 4
	realm = "thinkmay.net"

)


var (
    s *turn.Server
)

func Open(proxy_cred credential.Account,
         min_port int,
         max_port int) {
	uid,turn_cred,info, err := credential.SetupTurnAccount(proxy_cred,
        min_port,
        max_port)
	if err != nil {
		fmt.Printf("failed to setup turn account: %s", err.Error())
		return
	}

	s,err = SetupTurn(info.PublicIP,
        turn_cred.Username,
        turn_cred.Password, 
        info.Port,
        min_port,
        max_port)
	if err != nil {
		fmt.Printf("failed to setup turn account: %s", err.Error())
		return
	}

    if err != nil {
		fmt.Printf("failed to setup turn account: %s", err.Error())
		return
    }

	go func() {
		for {
			err := credential.Ping(uid)
			if err != nil { fmt.Println(err.Error()) }
			time.Sleep(10 * time.Second)
		}
	}()
}


func Close(){
    s.Close()
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

		fmt.Printf("[%s] Outbound STUN to %s: %s \n",time.Now().Format(time.RFC850),addr.String(), msg.String())
	}

	return
}

func (s *stunLogger) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if n, addr, err = s.PacketConn.ReadFrom(p); err == nil && stun.IsMessage(p) {
		msg := &stun.Message{Raw: p}
		if err = msg.Decode(); err != nil {
			return
		}

		fmt.Printf("[%s] Inbound  STUN to %s: %s \n",time.Now().Format(time.RFC850),addr.String(), msg.String())
	}

	return
}


func SetupTurn(publicip string,
				username string, 
				password string,
				port int, 
				min int, 
				max int) (*turn.Server, error){
	flag.Parse()

	// Create a UDP listener to pass into pion/turn
	// pion/turn itself doesn't allocate any UDP sockets, but lets the user pass them in
	// this allows us to add logging, storage or modify inbound/outbound traffic
	udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(port))
	if err != nil {
		log.Panicf("Failed to create TURN server listener: %s", err)
	}

	// Cache -users flag for easy lookup later
	// If passwords are stored they should be saved to your DB hashed using turn.GenerateAuthKey
	usersMap := map[string][]byte{}
	usersMap[username] = turn.GenerateAuthKey(username, realm, password)


	s, err := turn.NewServer(turn.ServerConfig{
		Realm: realm,
		// Set AuthHandler callback
		// This is called every time a user tries to authenticate with the TURN server
		// Return the key for that user, or false when no user is found
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			fmt.Printf("[%s] Incoming TURN: Request from %s\n",time.Now().Format(time.RFC850),srcAddr.String())
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
					Address:      "0.0.0.0",              // But actually be listening on every interface
					MinPort:      uint16(min),
					MaxPort:      uint16(max),
				},
			},
		},
	})

	return s,err
}



