package libvirt

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type VirtDaemon struct {
	libvirt    *Libvirt
}

func NewVirtDaemon() *VirtDaemon {
	return &VirtDaemon{
		libvirt:    NewLibvirt(),
	}
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


func (daemon *VirtDaemon) AttachDisk(body []byte) (any, error) {
	server := struct {
		ID      string      `yaml:"id"`
		Volumes []Volume    `yaml:"volumes"`
	}{}

	err := yaml.Unmarshal(body, &server)
	if err != nil {
		return nil, err
	}

	driver := "virtio"
	volumes := []Disk{}
	for i,v := range server.Volumes {
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


	err = daemon.libvirt.AttachDisk(
		server.ID,
		volumes,
	)

	return "SUCCESS", err
}

type Volume struct {
	Path    string  `yaml:"path"`
	Backing *Volume `yaml:"backing"`
}

func (daemon *VirtDaemon) DeployVM(body []byte) (any, error) {
	server := struct {
		ID      string      `yaml:"id"`
		VCPU    int         `yaml:"vcpus"`
		RAM     int         `yaml:"ram"`
		GPU     []GPU `yaml:"gpu"`
		Volumes []Volume    `yaml:"volumes"`
		VDriver bool		`yaml:"vdriver"`
		HideVM  bool		`yaml:"hidevm"`
		Pincpu  bool		`yaml:"pincpu"`
	}{}

	err := yaml.Unmarshal(body, &server)
	if err != nil {
		return nil, err
	}

	driver := "ide"
	if server.VDriver {
		driver = "virtio"
	}

	volumes := []Disk{}
	for i, v := range server.Volumes {
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

	name, err := daemon.libvirt.CreateVM(
		server.ID,
		server.VCPU,
		server.RAM,
		server.GPU,
		volumes,
	)

	return struct {
		Name string
	}{
		Name: name,
	}, err
}

func (daemon *VirtDaemon) DeleteVM(body []byte) (any, error) {
	server := struct {
		Name string `yaml:"name"`
	}{}

	err := yaml.Unmarshal(body, &server)
	if err != nil {
		return nil, err
	}

	err = daemon.libvirt.DeleteVM(server.Name)
	if err != nil {
		return nil, err
	}
	return fmt.Sprintf("VM %s deleted", server.Name), nil
}

func (daemon *VirtDaemon) StatusVM(body []byte) (any, error) {
	server := struct {
		Name string `yaml:"name"`
	}{}

	err := yaml.Unmarshal(body, &server)
	if err != nil {
		return nil, err
	}

	doms := daemon.libvirt.ListDomains()
	for _, d := range doms {
		if *d.Name == string(server.Name) {
			return struct { Status string }{ Status: *d.Status, }, nil
		}
	}

	return struct { Status string }{ Status: "StatusDeleted", }, nil
}

func (daemon *VirtDaemon) ListVMs(data []byte) (any, error) {
	doms := daemon.libvirt.ListDomains()

	result := map[string][]Domain{}

	for _, d := range doms {
		if d.Status == nil {
			unknown := "unknown"
			d.Status = &unknown
		}

		if result[*d.Status] == nil {
			result[*d.Status] = []Domain{d}
		} else {
			result[*d.Status] = append(result[*d.Status], d)
		}
	}

	for i, d := range result["StatusRunning"] {
		ips := daemon.libvirt.ListDomainIPs(d)
		result["StatusRunning"][i].PrivateIP = &ips
	}

	return result, nil
}

func (daemon *VirtDaemon) ListGPUs(data []byte) (any, error) {
	domains := daemon.libvirt.ListDomains()
	gpus := daemon.libvirt.ListGPUs()

	result := struct {
		Active    []GPU `yaml:"active"`
		Available []GPU `yaml:"open"`
	}{
		Active:    []GPU{},
		Available: []GPU{},
	}

	for _, g := range gpus {
		add := true
		for _, d := range domains {
			if (d.Status == nil) {
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
						add = false
					}
				}
			}
		}

		if add {
			result.Available = append(result.Available, g)
		}
		result.Active = append(result.Active, g)
	}

	return result, nil
}
