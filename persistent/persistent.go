package persistent

import "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"

type Persistent interface {
	Log(source string, level string, log string)
	Infor(log *packet.WorkerInfor)

	RecvSession() *packet.WorkerSessions
	SyncSession(log *packet.WorkerSessions)
}
