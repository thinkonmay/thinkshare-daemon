package libvirt

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
	"gopkg.in/yaml.v3"
)

type Libvirt struct {
	Version string
	conn    *libvirt.Libvirt
}

func NewLibvirt() (*Libvirt, error) {
	ret := &Libvirt{}

	c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to dial libvirt: %v", err)
	}

	ret.conn = libvirt.NewWithDialer(dialers.NewAlreadyConnected(c))
	if err := ret.conn.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	return ret, nil
}

func (lv *Libvirt) ListDomains() ([]Domain, error) {
	flags := libvirt.ConnectListDomainsActive | libvirt.ConnectListDomainsInactive
	domains, _, err := lv.conn.ConnectListAllDomains(1, flags)
	if err != nil {
		return []Domain{}, fmt.Errorf("failed to retrieve domains: %v", err)
	}

	ret := []Domain{}
	for _, d := range domains {
		desc, err := lv.conn.DomainGetXMLDesc(d, libvirt.DomainXMLSecure)
		if err != nil {
			continue
		}

		dom := Domain{}
		err = dom.Parse([]byte(desc))
		if err != nil {
			return []Domain{}, err
		}

		running, err := lv.conn.DomainIsActive(d)
		if err != nil {
			return []Domain{}, err
		}

		dom.Running = running == 1
		ret = append(ret, dom)
	}

	return ret, nil

}

func (lv *Libvirt) ListGPUs() ([]GPU, error) {
	dev, _, err := lv.conn.ConnectListAllNodeDevices(1, 0)
	if err != nil {
		return []GPU{}, err
	}

	ret := []GPU{}
	for _, nd := range dev {
		desc, err := lv.conn.NodeDeviceGetXMLDesc(nd.Name, 0)
		if err != nil {
			continue
		}

		gpu := GPU{}
		err = gpu.Parse([]byte(desc))
		if err != nil {
			return []GPU{}, err
		}

		vendor := strings.ToLower(gpu.Capability.Vendor.Val)
		if !strings.Contains(vendor, "nvidia") {
			continue
		}
		product := strings.ToLower(gpu.Capability.Product.Val)
		if strings.Contains(product, "audio") {
			continue
		}

		ret = append(ret, gpu)
	}

	return ret, nil
}

func (lv *Libvirt) CreateVM(id string,
	vcpus int,
	ram int,
	gpus []GPU,
	vols []Disk,
	ifaces []Interface,
) (libvirt.Domain, error) {
	dom := Domain{}
	err := yaml.Unmarshal([]byte(libvirtVM), &dom)
	if err != nil {
		return libvirt.Domain{},err
	}

	dom.Name = &id
	dom.Disk = vols
	dom.Memory.Value = ram * 1024 * 1024
	dom.CurrentMemory.Value = ram * 1024 * 1024
	dom.VCpu.Value = vcpus

	dom.Cpu.Topology.Sockets = 1
	// dom.Cpu.Topology.Clusters = 1
	dom.Cpu.Topology.Dies = 1
	dom.Cpu.Topology.Threads = 2
	dom.Cpu.Topology.Cores = vcpus / 2
	dom.Interfaces = ifaces

	dom.Hostdevs = []HostDev{}
	dom.Vcpupin = []Vcpupin{}

	for _, nd := range gpus {
		for _, v := range nd.Capability.IommuGroup.Address {
			dom.Hostdevs = append(dom.Hostdevs, HostDev{
				Mode:    "subsystem",
				Type:    "pci",
				Managed: "yes",
				SourceAddress: &struct {
					Domain   string "xml:\"domain,attr\""
					Bus      string "xml:\"bus,attr\""
					Slot     string "xml:\"slot,attr\""
					Function string "xml:\"function,attr\""
				}{
					Domain:   v.Domain,
					Bus:      v.Bus,
					Slot:     v.Slot,
					Function: v.Function,
				},
			})
		}
	}

	if err != nil {
		return libvirt.Domain{},err
	}

	xml, err := dom.ToString()
	if err != nil {
		return libvirt.Domain{},err
	}

	return lv.conn.DomainCreateXML(xml, libvirt.DomainStartValidate)
}

func (lv *Libvirt) AttachDisk(
	name string,
	vols []Disk) error {

	flags := libvirt.ConnectListDomainsActive
	doms, _, err := lv.conn.ConnectListAllDomains(1, flags)
	if err != nil {
		return err
	}

	dom := libvirt.Domain{Name: "null"}
	for _, d := range doms {
		if d.Name == name {
			dom = d
		}
	}

	if dom.Name == "null" {
		return fmt.Errorf("unknown VM name")
	}

	for _, d := range vols {
		err := lv.conn.DomainAttachDevice(dom, d.ToString())
		if err != nil {
			return err
		}
	}

	return nil
}

func (lv *Libvirt) DeleteVM(name string) error {
	if strings.Contains(name, "do-not-delete") {
		return nil
	}

	flags := libvirt.ConnectListDomainsActive
	doms, _, err := lv.conn.ConnectListAllDomains(1, flags)
	if err != nil {
		return err
	}

	dom := libvirt.Domain{Name: "null"}
	for _, d := range doms {
		if d.Name == name {
			dom = d
		}
	}

	if dom.Name == "null" {
		return fmt.Errorf("unknown VM name")
	}

	lv.conn.DomainDestroy(dom)
	lv.conn.DomainUndefine(dom)

	return nil
}
