package cluster

import (
	"fmt"
	"testing"
)

func TestCluster(t *testing.T) {
	config,err := NewClusterConfig("./manifest.yml")
	if err != nil {
		panic(err)
	}

	ifaces := config.Interface()
	fmt.Printf("%s\n",ifaces)

	for _,node := range config.Nodes() {
		gpus := node.GPUs()
		fmt.Printf("%v\n",gpus)
	}
}
