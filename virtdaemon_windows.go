package daemon

import (
	"fmt"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

type ClusterConfig struct {
}

func HandleVirtdaemon(*Daemon, *ClusterConfig) {
}

func (daemon *Daemon) DeployVM(g string) (*packet.WorkerInfor, error) {
	return nil, fmt.Errorf("window VM not available")
}
func (daemon *Daemon) ShutdownVM(*packet.WorkerInfor) error {
	return fmt.Errorf("window VM not available")
}

func InfoBuilder(info *packet.WorkerInfor) {
}

func HandleSessionForward(daemon *Daemon, ss *packet.WorkerSession, command string) (*packet.WorkerSession, error) {
	return nil,fmt.Errorf("window forward not available")
}