package ws

import (
	"fmt"
	"net/http"
	"strings"
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

	params := strings.Split(r.URL.RawQuery, "=")
	if len(params) == 1 {
		return
	}

	tenant := NewWsTenant(c)
	err = wsserver.fun(params[1], tenant)
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

func InitSignallingWs(port int) *WebSocketServer {
	wsserver := &WebSocketServer{}
	http.HandleFunc("/api/handshake", wsserver.HandleWebsocketSignaling)
	go http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", port), nil)
	return wsserver
}
