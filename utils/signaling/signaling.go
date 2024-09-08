package signaling

import (
	"fmt"
	"sync"

	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
)

type Signaling struct {
	client protocol.ProtocolHandler	

	mut      *sync.Mutex
	waitLine map[string]struct {
		server chan protocol.Tenant
		client chan protocol.Tenant
	}
}

func InitSignallingServer(client protocol.ProtocolHandler, server protocol.ProtocolHandler) *Signaling {
	signaling := Signaling{
		mut: &sync.Mutex{},
		client: client,
		waitLine: map[string]struct {
			server chan protocol.Tenant
			client chan protocol.Tenant
		}{},
	}

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
		signaling.mut.Lock()
		defer signaling.mut.Unlock()
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
		signaling.mut.Lock()
		defer signaling.mut.Unlock()
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

func (signaling *Signaling) AddSignalingChannel(token string) {
	signaling.mut.Lock()
	defer signaling.mut.Unlock()
	signaling.waitLine[token] = struct {
		server chan protocol.Tenant
		client chan protocol.Tenant
	}{
		server: make(chan protocol.Tenant, 8),
		client: make(chan protocol.Tenant, 8),
	}
}

func (signaling *Signaling) RemoveSignalingChannel(token string) {
	signaling.mut.Lock()
	defer signaling.mut.Unlock()
	delete(signaling.waitLine,token)
}
func (server *Signaling) AuthHandler(auth func(string) (*string,bool)) {
	server.client.AuthHandler(auth)
}