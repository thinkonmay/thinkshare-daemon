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

	// network, err := NewOVS("enp9s0")
	network, err := NewLibvirtNetwork("enp9s0")
	if err != nil {
		t.Error(err)
		return
	}
	// defer network.Close()

	for _, v := range vm {
		addr, err := network.FindDomainIPs(v)
		if err != nil {
			t.Error(err)
			return
		}

		fmt.Printf("%v\n", addr)

	}

	i, err := network.CreateInterface(Virtio)
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

	chain := Volume{"/home/huyhoang/thinkshare-daemon/utils/libvirt/31134452-4554-4fc8-a0ea-2c88e62ed17f.qcow2", nil}
	err = chain.PushChain(40)
	if err != nil {
		t.Error(err)
		return
	}

	chain2 := Volume{"/home/huyhoang/thinkshare-daemon/utils/libvirt/31134452-4554-4fc8-a0ea-2c88e62ed17f.qcow2", nil}
	err = chain2.PushChain(40)
	if err != nil {
		t.Error(err)
		return
	}

	_,err = d.DeployVM(VMLaunchModel{
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

	err = d.AttachDisk("test", []Volume{chain2})

	if err != nil {
		t.Error(err)
		return
	}
}
