package libvirt

import (
	"encoding/xml"
)

type Memory struct {
	Unit  string `xml:"unit,attr"`
	Value int    `xml:",chardata"`
}
type NumaTune struct {
	Memory struct {
		Mode    string `xml:"mode,attr"`
		Nodeset int    `xml:"nodeset,attr"`
	} `xml:"memory"`
}

type VCPU struct {
	Placement string `xml:"placement,attr"`
	Value     int    `xml:",chardata"`
}
type Vcpupin struct {
	Vcpu   int `xml:"vcpu,attr"`
	Cpuset int `xml:"cpuset,attr"`
}

type CPU struct {
	Mode       *string  `xml:"mode,attr"`
	Check      *string  `xml:"check,attr"`
	Migratable *string  `xml:"migratable,attr"`
	Topology   Topology `xml:"topology"`
	Feature    *struct {
		Policy *string `xml:"policy,attr"`
		Name   *string `xml:"name,attr"`
	} `xml:"feature"`
}

type Topology struct {
	Sockets  int `xml:"sockets,attr"`
	Dies     int `xml:"dies,attr"`
	// TODO : attempt this after virsh --version = 10.1.0
	// Clusters int `xml:"clusters,attr"`
	Cores    int `xml:"cores,attr"`
	Threads  int `xml:"threads,attr"`
}

type Resource struct {
	Partition *struct {
		Value *string `xml:",chardata"`
	} `xml:"partition"`
}

type OS struct {
	Boot *struct {
		Dev *string `xml:"dev,attr"`
	} `xml:"boot"`
	Type struct {
		Arch    *string `xml:"arch,attr"`
		Machine *string `xml:"machine,attr"`
		Value   *string `xml:",chardata"`
	} `xml:"type"`
	Smbios *struct {
		Mode *string `xml:"mode,attr"`
	} `xml:"smbios"`
}

type Features struct {
	Acpi   *struct{} `xml:"acpi"`
	Apic   *struct{} `xml:"apic"`
	Vmport *struct {
		State *string `xml:"state,attr"`
	} `xml:"vmport"`
	Kvm *struct {
		Hidden *struct {
			State *string `xml:"state,attr"`
		} `xml:"hidden"`
	} `xml:"kvm"`
}

type BackingStore struct {
	Type   string `xml:"type,attr"`
	Format *struct {
		Type string `xml:"type,attr"`
	} `xml:"format"`
	Source *struct {
		File string `xml:"file,attr"`
	} `xml:"source"`
	BackingStore *BackingStore `xml:"backingStore"`
}

func (domain *BackingStore) ToString() string {
	data, _ := xml.MarshalIndent(domain, "", "  ")
	return string(data)
}

type Disk struct {
	XMLName xml.Name `xml:"disk" yaml:"disk,inline"`
	Driver  *struct {
		Name string `xml:"name,attr"`
		Type string `xml:"type,attr"`
	} `xml:"driver"`
	Source *struct {
		File  string `xml:"file,attr"`
		Index int    `xml:"index,attr"`
	} `xml:"source"`
	Target *struct {
		Dev string `xml:"dev,attr"`
		Bus string `xml:"bus,attr"`
	} `xml:"target"`
	Address *struct {
		Type       *string `xml:"type,attr"`
		Controller *string `xml:"controller,attr"`
		Bus        *string `xml:"bus,attr"`
		Target     *string `xml:"target,attr"`
		Unit       *string `xml:"unit,attr"`
	} `xml:"address"`
	BackingStore *BackingStore `xml:"backingStore"`

	Type   string `xml:"type,attr"`
	Device string `xml:"device,attr"`
}

func (domain *Disk) ToString() string {
	data, _ := xml.MarshalIndent(domain, "", "  ")
	return string(data)
}

type Controller struct {
	Type  *string `xml:"type,attr"`
	Index *int    `xml:"index,attr"`
	Model *string `xml:"model,attr"`

	Master *struct {
		Startport *int `xml:"startport,attr"`
	} `xml:"master"`
	Address *struct {
		Type          *string `xml:"type,attr"`
		Domain        *string `xml:"domain,attr"`
		Bus           *string `xml:"bus,attr"`
		Slot          *string `xml:"slot,attr"`
		Function      *string `xml:"function,attr"`
		Multifunction *string `xml:"multifunction,attr"`
	} `xml:"address"`
}

type InterfaceSource struct {
	Dev     *string `xml:"dev,attr"`
	Mode    *string `xml:"mode,attr"`
	Network *string `xml:"network,attr"`
	Bridge  *string `xml:"bridge,attr"`
}
type InterfaceAddr struct {
	Type     *string `xml:"type,attr"`
	Domain   *string `xml:"domain,attr"`
	Bus      *string `xml:"bus,attr"`
	Slot     *string `xml:"slot,attr"`
	Function *string `xml:"function,attr"`
}
type Interface struct {
	Type        string  `xml:"type,attr"`
	Name        *string `xml:"name,attr"`
	VirtualPort *struct {
		Type string `xml:"type,attr"`
	} `xml:"virtualport"`

	Mac *struct {
		Address *string `xml:"address,attr"`
	} `xml:"mac"`
	Target *struct {
		Dev string `xml:"dev,attr"`
	} `xml:"target"`
	Source *InterfaceSource `xml:"source"`
	Model  *struct {
		Type *string `xml:"type,attr"`
	} `xml:"model"`
	Address *InterfaceAddr `xml:"address"`
}

type HostDev struct {
	Mode          string `xml:"mode,attr"`
	Type          string `xml:"type,attr"`
	Managed       string `xml:"managed,attr"`
	SourceAddress *struct {
		Domain   string `xml:"domain,attr"`
		Bus      string `xml:"bus,attr"`
		Slot     string `xml:"slot,attr"`
		Function string `xml:"function,attr"`
	} `xml:"source>address"`
	Address *struct {
		Type     string `xml:"type,attr"`
		Domain   string `xml:"domain,attr"`
		Bus      string `xml:"bus,attr"`
		Slot     string `xml:"slot,attr"`
		Function string `xml:"function,attr"`
	} `xml:"address"`
}

func (domain *HostDev) ToString() string {
	data, _ := xml.MarshalIndent(domain, "", "  ")
	return string(data)
}

type Emulator struct {
	Value *string `xml:",chardata"`
}

type Sound struct {
	Model        *string `xml:"model,attr"`
	SoundAddress *struct {
		Type     *string `xml:"type,attr"`
		Domain   *string `xml:"domain,attr"`
		Bus      *string `xml:"bus,attr"`
		Slot     *string `xml:"slot,attr"`
		Function *string `xml:"function,attr"`
	} `xml:"address"`
}

type Video struct {
	Model *struct {
		Ram     int    `xml:"ram,attr"`
		Vram    int    `xml:"vram,attr"`
		Vgamem  int    `xml:"vgamem,attr"`
		Heads   int    `xml:"heads,attr"`
		Type    string `xml:"type,attr"`
		Primary string `xml:"primary,attr"`
	} `xml:"model"`
	Address *struct {
		Type     string `xml:"type,attr"`
		Domain   string `xml:"domain,attr"`
		Bus      string `xml:"bus,attr"`
		Slot     string `xml:"slot,attr"`
		Function string `xml:"function,attr"`
	} `xml:"address"`
}

type Graphic struct {
	Type     string `xml:"type,attr"`
	Autoport string `xml:"autoport,attr"`
	Listen   *struct {
		Type string `xml:"type,attr"`
	} `xml:"listen"`
	Image *struct {
		Compression string `xml:"compression,attr"`
	} `xml:"image"`
}

type Channel struct {
	Type   string `xml:"type,attr"`
	Target struct {
		Type string `xml:"type,attr"`
		Name string `xml:"name,attr"`
	} `xml:"target"`
	Address struct {
		Type       string `xml:"type,attr"`
		Controller string `xml:"controller,attr"`
		Bus        string `xml:"bus,attr"`
		Port       string `xml:"port,attr"`
	} `xml:"address"`
}

type Memballoon struct {
	Model   string `xml:"model,attr"`
	Address *struct {
		Type     string `xml:"type,attr"`
		Domain   string `xml:"domain,attr"`
		Bus      string `xml:"bus,attr"`
		Slot     string `xml:"slot,attr"`
		Function string `xml:"function,attr"`
	} `xml:"address"`
}
type PM struct {
	SuspendToMem *struct {
		Enable string `xml:"enabled,attr"`
	} `xml:"suspend-to-mem"`
	SuspendToDisk *struct {
		Enable string `xml:"enabled,attr"`
	} `xml:"suspend-to-disk"`
}
type OnPoweroff struct {
	Value *string `xml:",chardata"`
}
type OnReboot struct {
	Value *string `xml:",chardata"`
}
type OnCrash struct {
	Value *string `xml:",chardata"`
}
type Input struct {
	Type string `xml:"type,attr"`
	Bus  string `xml:"bus,attr"`
}

type Clock struct {
	Offset string `xml:"offset,attr"`
	Timers []struct {
		Name       *string `xml:"name,attr"`
		Tickpolicy *string `xml:"tickpolicy,attr"`
		Present    *string `xml:"present,attr"`
	} `xml:"timer"`
}

type Domain struct {
	Running bool     `xml:"-"`
	XMLName xml.Name `xml:"domain" yaml:"domain,inline"`
	Type    *string  `xml:"type,attr"`

	Name *string `xml:"name"`
	Uuid *string `xml:"uuid"`

	NumaTune      *NumaTune `xml:"numatune"`
	Memory        Memory    `xml:"memory"`
	CurrentMemory Memory    `xml:"currentMemory"`
	VCpu          VCPU      `xml:"vcpu"`
	OS            OS        `xml:"os"`
	Features      Features  `xml:"features"`
	Cpu           CPU       `xml:"cpu"`
	Clock         Clock     `xml:"clock"`

	OnReboot   *OnReboot   `xml:"on_reboot"`
	OnPoweroff *OnPoweroff `xml:"on_poweroff"`
	OnCrash    *OnCrash    `xml:"on_crash"`
	PM         *PM         `xml:"pm"`

	Vcpupin []Vcpupin `xml:"cputune>vcpupin"`

	Emulator    *Emulator    `xml:"devices>emulator"`
	Disk        []Disk       `xml:"devices>disk"`
	Controllers []Controller `xml:"devices>controller"`
	Interfaces  []Interface  `xml:"devices>interface"`
	Channel     *Channel     `xml:"devices>channel"`
	Input       []Input      `xml:"devices>input"`
	Graphic     *Graphic     `xml:"devices>graphics"`
	Sound       *Sound       `xml:"devices>sound"`
	Video       *Video       `xml:"devices>video"`
	Hostdevs    []HostDev    `xml:"devices>hostdev"`
	Memballoon  *Memballoon  `xml:"devices>memballoon"`
}

func (domain *Domain) Parse(data []byte) error {
	return xml.Unmarshal(data, domain)
}
func (domain *Domain) ToString() (string, error) {
	data, err := xml.MarshalIndent(domain, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

type GPU struct {
	XMLName xml.Name `xml:"device" yaml:"domain,inline"`

	VM     *string // not mapped
	Active bool    // not mapped

	Name   string `xml:"name"`
	Path   string `xml:"path"`
	Parent string `xml:"parent"`

	Driver struct {
		Name string `xml:"name"`
	} `xml:"driver"`

	Capability struct {
		Type     string `xml:"type,attr"`
		Class    string `xml:"class"`
		Domain   string `xml:"domain"`
		Bus      string `xml:"bus"`
		Slot     string `xml:"slot"`
		Function string `xml:"function"`

		Product struct {
			Val string `xml:",chardata"`
			Id  string `xml:"id,attr"`
		} `xml:"product"`
		Vendor struct {
			Val string `xml:",chardata"`
			Id  string `xml:"id,attr"`
		} `xml:"vendor"`
		IommuGroup struct {
			Number  *int `xml:"number,attr"`
			Address []struct {
				Domain   string `xml:"domain,attr"`
				Bus      string `xml:"bus,attr"`
				Slot     string `xml:"slot,attr"`
				Function string `xml:"function,attr"`
			} `xml:"address"`
		} `xml:"iommuGroup"`
		Numa *struct {
			Node *int `xml:"node,attr"`
		} `xml:"numa"`
		Link []struct {
			Validity *string  `xml:"validity,attr"`
			Port     *float32 `xml:"port,attr"`
			Speed    *float32 `xml:"speed,attr"`
			Width    *float32 `xml:"width,attr"`
		} `xml:"pci-express>link"`
	} `xml:"capability"`
}

func (domain *GPU) Parse(data []byte) error {
	return xml.Unmarshal(data, domain)
}

type Iface struct {
	Type string `xml:"type,attr"`
	Name string `xml:"name,attr"`

	Source *struct {
		Dev  *string `xml:"dev,attr"`
		Mode *string `xml:"mode,attr"`
	} `xml:"source"`

	Mac *struct {
		Address *string `xml:"address,attr"`
	} `xml:"mac"`

	MTU *struct {
		Size *int `xml:"size,attr"`
	} `xml:"mtu"`

	Link *struct {
		State *string  `xml:"state,attr"`
		Speed *float64 `xml:"speed,attr"`
	} `xml:"link"`
}

func (iface *Iface) Parse(dat string) error {
	return xml.Unmarshal([]byte(dat), iface)
}

type StoragePool struct {
	XMLName xml.Name `xml:"pool"`
	Type    string   `xml:"type,attr"`
	Name    string   `xml:"name"`

	Path string `xml:"target>path"`
}

type VMLaunchModel struct {
	ID            string      `yaml:"id"`
	VCPU          int         `yaml:"vcpus"`
	RAM           int         `yaml:"ram"`
	GPU           []GPU       `yaml:"gpu"`
	BackingVolume []Volume    `yaml:"volumes"`
	Interfaces    []Interface `yaml:"volumes"`
	VDriver       bool        `yaml:"vdriver"`
}
