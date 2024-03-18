package daemon

import (
	"fmt"
	"testing"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)


func TestVirt(t *testing.T) {
	log.TakeLog(func(log string) {fmt.Println(log)})
	HandleVirtdaemon(&Daemon{vms: []*packet.WorkerInfor{}})	
}