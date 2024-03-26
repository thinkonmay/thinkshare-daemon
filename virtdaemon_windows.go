package daemon

import (
	"fmt"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

type ClusterConfig struct{}

func deinit()                                                {}
func QueryInfo(info *packet.WorkerInfor)                     {}
func InfoBuilder(info packet.WorkerInfor) packet.WorkerInfor { return info }

func (daemon *Daemon) HandleVirtdaemon(*ClusterConfig) {}
func (daemon *Daemon) DeployVM(g string) (*packet.WorkerInfor, error) {
	return nil, fmt.Errorf("window VM not available")
}
func (daemon *Daemon) ShutdownVM(*packet.WorkerInfor) error {
	return fmt.Errorf("window VM not available")
}
func (daemon *Daemon) HandleSessionForward(ss *packet.WorkerSession, command string) (*packet.WorkerSession, error) {
	return nil, fmt.Errorf("window forward not available")
}
