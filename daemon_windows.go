package daemon

import (
	"fmt"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

func (daemon *Daemon) HandleVirtdaemon() func() { return func() {} }
func (daemon *Daemon) DeployVM(_ *packet.WorkerSession, _,_ chan bool) (*packet.WorkerInfor, error) {
	return nil, fmt.Errorf("window VM not available")
}
func (daemon *Daemon) ShutdownVM(*packet.WorkerInfor) error {
	return fmt.Errorf("window VM not available")
}
func (daemon *Daemon) DeployVMwithVolume(nss *packet.WorkerSession, _,_ chan bool) (*packet.WorkerSession, *packet.WorkerInfor, error) {
	return nil, nil, fmt.Errorf("window forward not available")
}
func queryLocal(info *packet.WorkerInfor) error {
	return nil
}
func querySession(session *packet.WorkerSession) error {
	return nil
}
