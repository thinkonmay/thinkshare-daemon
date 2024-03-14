package ws

import (
	"fmt"

	"github.com/thinkonmay/thinkremote-rtchub/signalling/gRPC/packet"
)

type HttpTenant struct {
	exited    bool
	Outcoming chan *packet.SignalingMessage
	Incoming  chan *packet.SignalingMessage
}

func NewWsTenant(id string) *HttpTenant {
	return &HttpTenant{
		Outcoming: make(chan *packet.SignalingMessage, 5),
		Incoming:  make(chan *packet.SignalingMessage, 5),
		exited:    false,
	}
}

func (tenant *HttpTenant) Send(pkt *packet.SignalingMessage) {
	tenant.Outcoming <- pkt
}

func (tenant *HttpTenant) Receive() *packet.SignalingMessage {
	return <-tenant.Incoming
}

func (tenant *HttpTenant) Peek() bool {
	return len(tenant.Incoming) > 0
}

func (tenant *HttpTenant) Exit() {
	fmt.Printf("websocket tenant closed\n")
	tenant.exited = true
	tenant.Incoming<-nil
}
func (tenant *HttpTenant) IsExited() bool {
	return tenant.exited 
}
