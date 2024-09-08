package ws

import (
	"github.com/thinkonmay/thinkremote-rtchub/signalling/gRPC/packet"
)

type HttpTenant struct {
	exited    bool
	Outcoming chan *packet.SignalingMessage
	Incoming  chan *packet.SignalingMessage
}

func NewWsTenant(id string) *HttpTenant {
	return &HttpTenant{
		Outcoming: make(chan *packet.SignalingMessage, 64),
		Incoming:  make(chan *packet.SignalingMessage, 64),
		exited:    false,
	}
}

func (tenant *HttpTenant) Send(pkt *packet.SignalingMessage) {
	tenant.Outcoming <- pkt
}

func (tenant *HttpTenant) Receive() *packet.SignalingMessage {
	return <-tenant.Incoming
}

func (tenant *HttpTenant) Exit() {
	tenant.exited = true
	tenant.Incoming <- nil
}
func (tenant *HttpTenant) IsExited() bool {
	return tenant.exited
}
