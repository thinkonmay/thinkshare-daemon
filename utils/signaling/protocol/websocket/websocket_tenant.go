package ws

import (
	"encoding/json"
	"fmt"

	"github.com/gorilla/websocket"
	"github.com/thinkonmay/thinkremote-rtchub/signalling/gRPC/packet"
)

type WebsocketTenant struct {
	exited bool
	conn   *websocket.Conn

	pending chan *packet.SignalingMessage
}


func NewWsTenant(conn *websocket.Conn) *WebsocketTenant {
	ret := &WebsocketTenant{
		pending : make(chan *packet.SignalingMessage, 5),
		conn: conn,
		exited: false,
	}

	go func ()  {
		for {
			msgt, data, err := ret.conn.ReadMessage()
			if err != nil || msgt == websocket.CloseMessage{
				ret.Exit()
				return 
			}

			req := packet.SignalingMessage{}
			err = json.Unmarshal(data, &req)
			if err != nil {
				continue
			}

			ret.pending <-&req
		}
	}()

	return ret;
}

func (tenant *WebsocketTenant) Send(pkt *packet.SignalingMessage) {
	if pkt == nil {
		return
	}
	data, err := json.Marshal(pkt)
	if err != nil {
		return
	}
	err = tenant.conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		tenant.Exit()
	}
}

func (tenant *WebsocketTenant) Receive() *packet.SignalingMessage {
	return <-tenant.pending
}

func (tenant *WebsocketTenant) Peek() bool {
	return len(tenant.pending) > 0
}

func (tenant *WebsocketTenant) Exit() {
	fmt.Printf("websocket tenant closed\n")
	tenant.pending<-nil
	tenant.conn.Close()
	tenant.exited = true
}

func (tenant *WebsocketTenant) IsExited() bool {
	return tenant.exited
}