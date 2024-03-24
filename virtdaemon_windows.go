package daemon

import (
	"fmt"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

func HandleVirtdaemon(daemon *Daemon) {
}

func (daemon *Daemon)DeployVM(g string) (*packet.WorkerInfor, error) {
	return nil, fmt.Errorf("window VM not available")
}
func (daemon *Daemon)ShutdownVM(*packet.WorkerInfor) error {
	return fmt.Errorf("window VM not available")
}
