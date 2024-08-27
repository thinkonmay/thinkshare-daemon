package app

import (
	"os/exec"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

func StartApp(path string, args ...string) {
	exec.Command(path, args...).Start()
}

func GetKeepaliveID(msg *packet.WorkerSession) (keepaliveid string) {
	if msg.Vm != nil && msg.Vm.Volumes != nil && len(msg.Vm.Volumes) > 0 {
		keepaliveid = msg.Vm.Volumes[0]
	} else {
		keepaliveid = msg.Id
	}

	return
}
