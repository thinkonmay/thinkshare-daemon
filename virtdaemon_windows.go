package daemon

import (
	"fmt"

	"github.com/thinkonmay/thinkshare-daemon/utils/libvirt"
)

func HandleVirtdaemon(daemon *Daemon) {
}

func DeployVM(g string) (*libvirt.VMLaunchModel ,error) {
	return nil,fmt.Errorf("window VM not available")
}
func ShutdownVM(model libvirt.VMLaunchModel) error {
	return fmt.Errorf("window VM not available")
}