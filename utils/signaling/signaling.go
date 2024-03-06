package signaling

import (
	"time"

	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
)

type Signalling struct {
	client_waitLine chan protocol.Tenant
	server_waitLine chan protocol.Tenant
}

func InitSignallingServer(client protocol.ProtocolHandler, server protocol.ProtocolHandler) *Signalling {
	signaling := Signalling{
		client_waitLine: make(chan protocol.Tenant, 8),
		server_waitLine: make(chan protocol.Tenant, 8),
	}
	go func() {
		for {
			client := []protocol.Tenant{}
			server := []protocol.Tenant{}
			for {
				if len(signaling.client_waitLine) == 0 {
					break
				}
				wait := <-signaling.client_waitLine
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

			for {
				if len(signaling.server_waitLine) == 0 {
					break
				}
				wait := <-signaling.server_waitLine
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

			for _, t := range client {
				signaling.client_waitLine <- t
			}
			for _, t := range server {
				signaling.server_waitLine <- t
			}
			time.Sleep(time.Second)
		}
	}()

	server.OnTenant(func(tent protocol.Tenant) error {
		if len(signaling.client_waitLine) == 0 {
			signaling.server_waitLine <- tent
			return nil
		} else {
			pair := &Pair{<-signaling.client_waitLine, tent}
			pair.handlePair()
		}

		return nil
	})
	client.OnTenant(func(tent protocol.Tenant) error {
		if len(signaling.server_waitLine) == 0 {
			signaling.client_waitLine <- tent
		} else {
			pair := Pair{<-signaling.server_waitLine, tent}
			pair.handlePair()
		}

		return nil
	})

	return &signaling
}
