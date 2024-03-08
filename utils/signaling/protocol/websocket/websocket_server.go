package ws

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
)

type WebSocketServer struct {
	fun  protocol.OnTenantFunc
	auth func(*http.Request) bool
	path string
}

func (server *WebSocketServer) OnTenant(fun protocol.OnTenantFunc) {
	server.fun = fun
}

func (wsserver *WebSocketServer) HandleWebsocketSignaling(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {return true},
	}
	if !wsserver.auth(r) {
		w.WriteHeader(401)
		return
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.PushLog(err.Error())
		return
	}

	tenant := NewWsTenant(c)
	err = wsserver.fun(protocol.Tenant{ tenant,r.URL.Query().Get("token"), })
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
		fun:  func(protocol.Tenant) error { return nil },
		auth: auth,
		path: path,
	}
	http.HandleFunc(path, wsserver.HandleWebsocketSignaling)
	return wsserver
}
