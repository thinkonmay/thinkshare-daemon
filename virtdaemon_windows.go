package daemon

import (
	"fmt"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

type ClusterConfig struct{}

func deinit()                                               {}
func QueryInfo(info *packet.WorkerInfor) packet.WorkerInfor { return *info }

func (daemon *Daemon) HandleVirtdaemon(*ClusterConfig) func() { return func() {} }
func (daemon *Daemon) DeployVM(*packet.WorkerSession, chan bool) (*packet.WorkerInfor, error) {
	return nil, fmt.Errorf("window VM not available")
}
func (daemon *Daemon) ShutdownVM(*packet.WorkerInfor) error {
	return fmt.Errorf("window VM not available")
}
func (daemon *Daemon) HandleSessionForward(ss *packet.WorkerSession, command string) (*packet.WorkerSession, error) {
	return nil, fmt.Errorf("window forward not available")
}

func (daemon *Daemon) HandleSignaling(token string) (*string, bool) {
	return nil, false
}
func (daemon *Daemon) DeployVMwithVolume(nss *packet.WorkerSession, _ chan bool) (*packet.WorkerSession, *packet.WorkerInfor, error) {
	return nil, nil, fmt.Errorf("window forward not available")
}
