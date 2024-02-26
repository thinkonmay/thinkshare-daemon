package persistent

import "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"

type Persistent interface {
	Log(source string, level string, log string)
	Infor(log *packet.WorkerInfor)

	RecvSession() *packet.WorkerSession
	ClosedSession() int
	FailedSession(log *packet.WorkerSession)
}
