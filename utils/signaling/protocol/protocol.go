package protocol

import "github.com/thinkonmay/thinkremote-rtchub/signalling/gRPC/packet"

type ITenant interface {
	Send(*packet.SignalingMessage)
	Receive() *packet.SignalingMessage

	IsExited() bool
	Exit()
}

type Tenant struct {
	ITenant
	Token string
}

type OnTenantFunc func(tent Tenant) error

type ProtocolHandler interface {
	OnTenant(fun OnTenantFunc)
	AuthHandler(auth func(string) *string)
}
