package persistent

import "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"

type Persistent interface {
	Log(source string, level string, log string)
	Infor(log *packet.WorkerInfor)
	Sessions(func ()[]packet.WorkerSession)

	RecvSession(func(*packet.WorkerSession) error)
	ClosedSession() int
	Stop()
}
