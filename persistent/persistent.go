package persistent

import "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"

type Persistent interface {
	Log(source string, level string, log string)
	Infor(func () *packet.WorkerInfor)
	RecvSession(func(*packet.WorkerSession,chan bool) (*packet.WorkerSession,error))
	ClosedSession(func(*packet.WorkerSession) error) 
	Stop()
}
