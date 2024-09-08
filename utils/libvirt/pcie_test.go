package libvirt

import (
	"fmt"
	"testing"

	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

func TestPCIE(t *testing.T) {
	log.TakeLog(func(log string) {
		fmt.Println(log)
	})

	lv, err := NewVirtDaemon()
	if err != nil {
		panic(err)
	}

	gs, err := lv.ListGPUs()
	if err != nil {
		panic(err)
	}

	for _, g := range gs {
		fmt.Printf("%v\n", g.Name)
	}
}
