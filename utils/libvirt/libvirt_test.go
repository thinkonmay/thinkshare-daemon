package libvirt

import (
	"fmt"
	"testing"
)

func TestLibvirt(t *testing.T) {
	d, err := NewVirtDaemon()
	if err != nil {
		t.Error(err)
		return
	}

	g, err := d.ListGPUs()
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Printf("%v\n", g)

	vm, err := d.ListVMs()
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Printf("%v\n", vm)

	network, err := NewOVS("enp9s0")
	if err != nil {
		t.Error(err)
		return
	}
	// defer network.Close()

	i, err := network.CreateInterface(driver_virtio)
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Printf("%v\n", i)

	gpus, err := d.ListGPUs()
	if err != nil {
		t.Error(err)
		return
	}

	chain, err := d.GrowChain(Volume{Path: "/home/huyhoang/thinkshare-daemon/utils/libvirt/31134452-4554-4fc8-a0ea-2c88e62ed17f.qcow2"}, 40)
	if err != nil {
		t.Error(err)
		return
	}

	err = d.DeployVM(VMLaunchModel{
		ID:            "test",
		VCPU:          8,
		RAM:           8,
		GPU:           gpus,
		BackingVolume: []Volume{chain},
		Interfaces:    []Interface{*i},
		VDriver:       true,
	})

	if err != nil {
		t.Error(err)
		return
	}
}
