package libvirt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

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

type Volume struct {
	Path    string  `yaml:"path"`
	Backing *Volume `yaml:"backing"`
}

func (daemon *VirtDaemon) DeployVM(server VMLaunchModel) error {
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

	return daemon.libvirt.CreateVM(
		server.ID,
		server.VCPU,
		server.RAM,
		server.GPU,
		volumes,
		server.Interfaces,
	)
}

func (daemon *VirtDaemon) GrowChain(chain Volume, size int) (Volume, error) {
	_, err := os.Stat(chain.Path)
	if err != nil {
		return Volume{}, err
	}

	
	now := uuid.NewString()
	dir := filepath.Dir(chain.Path)
	path := fmt.Sprintf("%s/%s.qcow2",dir,now)
	_,err = exec.Command("/usr/bin/qemu-img","create", "-f", "qcow2", "-F", "qcow2", "-o",
	    fmt.Sprintf("backing_file=%s",chain.Path), path,
	    fmt.Sprintf("%dG",size)).Output()
	if err != nil {
		return Volume{}, err
	}


	return Volume{
		Path: path,
		Backing: &chain,
	},nil
}

func (daemon *VirtDaemon) DeleteVM(name string) error {
	err := daemon.libvirt.DeleteVM(name)
	if err != nil {
		return err
	}
	return nil
}

func (daemon *VirtDaemon) StatusVM(name string) (any, error) {
	doms, err := daemon.libvirt.ListDomains()
	if err != nil {
		return nil, err
	}
	for _, d := range doms {
		if *d.Name == name {
			return struct{ Status string }{Status: *d.Status}, nil
		}
	}

	return struct{ Status string }{Status: "StatusDeleted"}, nil
}

func (daemon *VirtDaemon) ListVMs() ([]Domain, error) {
	doms, err := daemon.libvirt.ListDomains()
	if err != nil {
		return nil, err
	}

	result := []Domain{}
	for _, d := range doms {
		if d.Status == nil {
			unknown := "unknown"
			d.Status = &unknown
		}

		result = append(result, d)
	}

	return result, nil
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
			if d.Status == nil {
				continue
			} else if *d.Status == "StatusShutdown" {
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
