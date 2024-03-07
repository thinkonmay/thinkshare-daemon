package ws

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
)

type WebSocketServer struct {
	fun  protocol.OnTenantFunc
	auth func(*http.Request) bool
}

func (server *WebSocketServer) OnTenant(fun protocol.OnTenantFunc) {
	server.fun = fun
}

func (wsserver *WebSocketServer) HandleWebsocketSignaling(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{}
	if !wsserver.auth(r) {
		w.WriteHeader(401)
		return
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(503)
		return
	}

	tenant := NewWsTenant(c)
	err = wsserver.fun(tenant)
	if err != nil {
		tenant.Exit()
	}

	for {
		time.Sleep(100 * time.Millisecond)
		if tenant.IsExited() {
			return
		}
	}
}

func InitSignallingWs(path string, auth func(*http.Request) bool) *WebSocketServer {
	wsserver := &WebSocketServer{
		fun:  func(tent protocol.Tenant) error { return nil },
		auth: auth,
	}
	http.HandleFunc(path, wsserver.HandleWebsocketSignaling)
	return wsserver
}
