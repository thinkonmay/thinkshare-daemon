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

	config, err := NewClusterConfig("./manifest.yml")
	if err != nil {
		panic(err)
	}

	ifaces := config.Interface()
	fmt.Printf("%s\n", ifaces)

	for {
		for _, node := range config.Nodes() {
			node.Query()
			gpus := node.GPUs()
			fmt.Printf("%v\n", gpus)
		}
		for _, node := range config.Peers() {
			node.Query()
			gpus := node.GPUs()
			fmt.Printf("%v\n", gpus)
		}
		time.Sleep(time.Second)
	}
}
