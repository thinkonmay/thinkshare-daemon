package signaling

import (
	"fmt"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
)

type Signalling struct {
	waitLine map[string]struct {
		server chan protocol.Tenant
		client chan protocol.Tenant
	}
}

func InitSignallingServer(client protocol.ProtocolHandler, server protocol.ProtocolHandler) *Signalling {
	signaling := Signalling{
		waitLine: map[string]struct {
			server chan protocol.Tenant
			client chan protocol.Tenant
		}{
			"video": {
				server: make(chan protocol.Tenant, 8),
				client: make(chan protocol.Tenant, 8),
			},
			"audio": {
				server: make(chan protocol.Tenant, 8),
				client: make(chan protocol.Tenant, 8),
			},
		},
	}
	go func() {
		for {
			client := []protocol.Tenant{}
			server := []protocol.Tenant{}
			for _, v := range signaling.waitLine {
				for {
					if len(v.client) == 0 {
						break
					}
					wait := <-v.client
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
					if len(v.server) == 0 {
						break
					}
					wait := <-v.server
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

			for _, t := range client {
				signaling.waitLine[t.Token].client <- t
			}
			for _, t := range server {
				signaling.waitLine[t.Token].server <- t
			}
			time.Sleep(time.Second)
		}
	}()

	find_token := func(token string) bool {
		keys := make([]string, 0, len(signaling.waitLine))
		for k := range signaling.waitLine {
			keys = append(keys, k)
		}

		found := false
		for _, v := range keys {
			if v == token {
				found = true
			}
		}

		return found
	}

	server.OnTenant(func(tent protocol.Tenant) error {
		if !find_token(tent.Token) {
			return fmt.Errorf("invalid key %s", tent.Token)
		}

		if len(signaling.waitLine[tent.Token].client) == 0 {
			signaling.waitLine[tent.Token].server <- tent
			return nil
		} else {
			pair := &Pair{<-signaling.waitLine[tent.Token].client, tent}
			pair.handlePair()
		}

		return nil
	})
	client.OnTenant(func(tent protocol.Tenant) error {
		if !find_token(tent.Token) {
			return fmt.Errorf("invalid key %s", tent.Token)
		}

		if len(signaling.waitLine[tent.Token].server) == 0 {
			signaling.waitLine[tent.Token].client <- tent
		} else {
			pair := Pair{<-signaling.waitLine[tent.Token].server, tent}
			pair.handlePair()
		}

		return nil
	})

	return &signaling
}

func (signaling *Signalling) AddSignalingChannel(token string) {
	signaling.waitLine[token] = struct {
		server chan protocol.Tenant
		client chan protocol.Tenant
	}{
		server: make(chan protocol.Tenant, 8),
		client: make(chan protocol.Tenant, 8),
	}
}
