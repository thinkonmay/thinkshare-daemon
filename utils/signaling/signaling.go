package signaling

import (
	"fmt"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
)

type Signalling struct {
	client_waitLine map[string]chan protocol.Tenant
	server_waitLine map[string]chan protocol.Tenant
}

func InitSignallingServer(client protocol.ProtocolHandler, server protocol.ProtocolHandler) *Signalling {
	signaling := Signalling{
		client_waitLine: map[string]chan protocol.Tenant{
			"video": make(chan protocol.Tenant, 8),
			"audio": make(chan protocol.Tenant, 8),
		},
		server_waitLine: map[string]chan protocol.Tenant{
			"video": make(chan protocol.Tenant, 8),
			"audio": make(chan protocol.Tenant, 8),
		},
	}
	go func() {
		for {
			client := []protocol.Tenant{}
			server := []protocol.Tenant{}
			for _, v := range signaling.server_waitLine {
				for {
					if len(v) == 0 {
						break
					}
					wait := <-v
					if !wait.IsExited() {
						for {
							if !wait.Peek() {
								break
							}
							wait.Receive()
						}
						server = append(server, wait)
					}
				}
			}			
			for _, v := range signaling.client_waitLine {
				for {
					if len(v) == 0 {
						break
					}
					wait := <-v
					if !wait.IsExited() {
						for {
							if !wait.Peek() {
								break
							}
							wait.Receive()
						}
						client = append(client, wait)
					}
				}
			}

			for _, t := range client {
				signaling.client_waitLine[t.Token] <- t
			}
			for _, t := range server {
				signaling.server_waitLine[t.Token] <- t
			}
			time.Sleep(time.Second)
		}
	}()

	server.OnTenant(func(tent protocol.Tenant) error {
		keys := make([]string, 0, len(signaling.server_waitLine))
		for k := range signaling.server_waitLine {
			keys = append(keys, k)
		}

		found := false
		for _, v := range keys {
			if v == tent.Token {
				found = true
			}
		}

		if !found {
			return fmt.Errorf("invalid key %s",tent.Token)
		}

		if len(signaling.client_waitLine[tent.Token]) == 0 {
			signaling.server_waitLine[tent.Token] <- tent
			return nil
		} else {
			pair := &Pair{<-signaling.client_waitLine[tent.Token], tent}
			pair.handlePair()
		}

		return nil
	})
	client.OnTenant(func(tent protocol.Tenant) error {
		keys := make([]string, 0, len(signaling.client_waitLine))
		for k := range signaling.client_waitLine {
			keys = append(keys, k)
		}

		found := false
		for _, v := range keys {
			if v == tent.Token {
				found = true
			}
		}

		if !found {
			return fmt.Errorf("invalid key %s",tent.Token)
		}
		

		if len(signaling.server_waitLine[tent.Token]) == 0 {
			signaling.client_waitLine[tent.Token] <- tent
		} else {
			pair := Pair{<-signaling.server_waitLine[tent.Token], tent}
			pair.handlePair()
		}

		return nil
	})

	return &signaling
}

func (signaling *Signalling)AddSignalingChannel(token string) {
}