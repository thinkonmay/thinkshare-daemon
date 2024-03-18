package libvirt

import "fmt"

type VirtDaemon struct {
	libvirt *Libvirt
}

func NewVirtDaemon() (*VirtDaemon, error) {
	libvirt, err := NewLibvirt()
	if err != nil {
		return nil, err
	}

	return &VirtDaemon{
		libvirt,
	}, nil
}

func backingChain(vols *Volume) *BackingStore {
	if vols == nil {
		return nil
	}

	return &BackingStore{
		Type: "file",
		Format: &struct {
			Type string "xml:\"type,attr\""
		}{
			Type: "qcow2",
		},
		Source: &struct {
			File string "xml:\"file,attr\""
		}{
			File: vols.Path,
		},
		BackingStore: backingChain(vols.Backing),
	}
}

func (daemon *VirtDaemon) AttachDisk(vm string, Volumes []Volume) error {
	driver := "virtio"
	volumes := []Disk{}
	for i, v := range Volumes {
		dev := ""
		switch i {
		case 0:
			dev = "vdb"
		case 1:
			dev = "vdc"
		case 2:
			dev = "vdd"
		case 3:
			dev = "vde"
		}

		volumes = append(volumes, Disk{
			Driver: &struct {
				Name string "xml:\"name,attr\""
				Type string "xml:\"type,attr\""
			}{
				Name: "qemu",
				Type: "qcow2",
			},
			Source: &struct {
				File  string "xml:\"file,attr\""
				Index int    "xml:\"index,attr\""
			}{
				File:  v.Path,
				Index: 1,
			},
			Target: &struct {
				Dev string "xml:\"dev,attr\""
				Bus string "xml:\"bus,attr\""
			}{
				Dev: dev,
				Bus: driver,
			},
			Address:      nil,
			Type:         "file",
			Device:       "disk",
			BackingStore: backingChain(v.Backing),
		})
	}

	return daemon.libvirt.AttachDisk(
		vm,
		volumes,
	)
}

func (daemon *VirtDaemon) DeployVM(server VMLaunchModel) (Domain, error) {
	driver := "ide"
	if server.VDriver {
		driver = "virtio"
	}

	volumes := []Disk{}
	for i, v := range server.BackingVolume {
		dev := ""
		switch i {
		case 0:
			dev = "vda"
		case 1:
			dev = "vdb"
		case 2:
			dev = "vdc"
		case 3:
			dev = "vdd"
		}

		volumes = append(volumes, Disk{
			Driver: &struct {
				Name string "xml:\"name,attr\""
				Type string "xml:\"type,attr\""
			}{
				Name: "qemu",
				Type: "qcow2",
			},
			Source: &struct {
				File  string "xml:\"file,attr\""
				Index int    "xml:\"index,attr\""
			}{
				File:  v.Path,
				Index: 1,
			},
			Target: &struct {
				Dev string "xml:\"dev,attr\""
				Bus string "xml:\"bus,attr\""
			}{
				Dev: dev,
				Bus: driver,
			},
			Address:      nil,
			Type:         "file",
			Device:       "disk",
			BackingStore: backingChain(v.Backing),
		})
	}

	dom,err :=  daemon.libvirt.CreateVM(
		server.ID,
		server.VCPU,
		server.RAM,
		server.GPU,
		volumes,
		server.Interfaces,
	)
	if err != nil {
		return Domain{}, err
	}

	doms,err := daemon.libvirt.ListDomains()
	if err != nil {
		return Domain{}, err
	}

	for _,d  := range doms {
		if *d.Name == dom.Name {
			return d,nil
		}
	}

	return Domain{},fmt.Errorf("domain not found")
}

func (daemon *VirtDaemon) DeleteVM(name string) error {
	err := daemon.libvirt.DeleteVM(name)
	if err != nil {
		return err
	}
	return nil
}

func (daemon *VirtDaemon) ListVMs() ([]Domain, error) {
	return daemon.libvirt.ListDomains()
}

func (daemon *VirtDaemon) ListGPUs() ([]GPU, error) {
	domains, err := daemon.libvirt.ListDomains()
	if err != nil {
		return nil, err
	}
	gpus, err := daemon.libvirt.ListGPUs()
	if err != nil {
		return nil, err
	}

	result := []GPU{}
	for _, g := range gpus {
		for _, d := range domains {
			if !d.Running {
				continue
			}

			for _, hd := range d.Hostdevs {
				for _, v := range g.Capability.IommuGroup.Address {
					if hd.SourceAddress.Bus == v.Bus &&
						hd.SourceAddress.Domain == v.Domain &&
						hd.SourceAddress.Function == v.Function &&
						hd.SourceAddress.Slot == v.Slot {
						g.VM = d.Name
						g.Active = true
					}
				}
			}
		}

		result = append(result, g)
	}

	return result, nil
}
