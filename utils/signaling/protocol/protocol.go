package protocol

import "github.com/thinkonmay/thinkremote-rtchub/signalling/gRPC/packet"

type Tenant interface {
	Send(*packet.SignalingMessage)
	Receive() *packet.SignalingMessage
	Peek() bool

	IsExited() bool
	Exit()
}

type OnTenantFunc func(token string, tent Tenant) error

type ProtocolHandler interface {
	OnTenant(fun OnTenantFunc)
}