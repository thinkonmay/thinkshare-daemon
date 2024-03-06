package ws

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol"
)

type WebSocketServer struct {
	fun protocol.OnTenantFunc
}

func (server *WebSocketServer) OnTenant(fun protocol.OnTenantFunc) {
	server.fun = fun
}

func (wsserver *WebSocketServer) HandleWebsocketSignaling(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		return
	}

	tenant := NewWsTenant(c)
	err = wsserver.fun(tenant)
	if err != nil {
		tenant.Exit()
	}

	for {
		if tenant.IsExited() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func InitSignallingWs(path string) *WebSocketServer {
	wsserver := &WebSocketServer{}
	http.HandleFunc(path, wsserver.HandleWebsocketSignaling)
	return wsserver
}
