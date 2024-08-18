package libvirt

import (
	"fmt"
	"testing"
)

func TestPCIE(t *testing.T) {
	lv, err := NewLibvirt()
	if err != nil {
		panic(err)
	}

	gs, err := lv.ListGPUs()
	if err != nil {
		panic(err)
	}

	for _, g := range gs {
		fmt.Printf("%v\n", g)
	}
}
