package persistent

import "github.com/thinkonmay/conductor/protocol/gRPC/packet"

type Persistent interface {
	Log(source string,level string, log string)

	Metric(log *packet.WorkerMetric)
	Infor(log *packet.WorkerInfor)
	Media(log *packet.MediaDevice)


	RecvSession()*packet.WorkerSessions
	SyncSession(log *packet.WorkerSessions)
}