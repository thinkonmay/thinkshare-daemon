package cluster

import (
	"fmt"
	"testing"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

func TestCluster(t *testing.T) {
	log.TakeLog(func(log string) {
		str := fmt.Sprintf("daemon.exe : %s", log)
		fmt.Println(str)
	})

	_, err := NewClusterConfig("./manifest.yaml")
	if err != nil {
		panic(err)
	}

	for {
		time.Sleep(time.Second)
	}
}
