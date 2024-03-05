package libvirt

import (
	"fmt"
	"log"
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

	network Network
}

func NewLibvirt() *Libvirt {
	ret := &Libvirt{}

	ret.network = NewLibvirtNetwork()
	c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
	if err != nil {
		log.Fatalf("failed to dial libvirt: %v", err)
	}

	ret.conn = libvirt.NewWithDialer(dialers.NewAlreadyConnected(c))
	if err := ret.conn.Connect(); err != nil {
		log.Fatalf("failed to connect: %v", err)
	}

	return ret
}

func (lv *Libvirt) ListDomains() []Domain {
	flags := libvirt.ConnectListDomainsActive | libvirt.ConnectListDomainsInactive
	domains, _, err := lv.conn.ConnectListAllDomains(1, flags)
	if err != nil {
		log.Fatalf("failed to retrieve domains: %v", err)
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
			panic(err)
		}

		active, err := lv.conn.DomainIsActive(d)
		if err != nil {
			panic(err)
		}

		if active == 1 {
			status := "StatusRunning"
			dom.Status = &status
		} else {
			status := "StatusShutdown"
			dom.Status = &status

		}

		ret = append(ret, dom)
	}

	return ret

}

func (lv *Libvirt) ListGPUs() []GPU {
	dev, _, _ := lv.conn.ConnectListAllNodeDevices(1, 0)

	ret := []GPU{}
	for _, nd := range dev {
		desc, err := lv.conn.NodeDeviceGetXMLDesc(nd.Name, 0)
		if err != nil {
			continue
		}

		gpu := GPU{}
		err = gpu.Parse([]byte(desc))
		if err != nil {
			panic(err)
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

	return ret
}

func (lv *Libvirt) ListDomainIPs(dom Domain) []string { // TODO
	return lv.network.FindDomainIPs(dom)
}

func (lv *Libvirt) CreateVM(id string,
	vcpus int,
	ram int,
	gpus []GPU,
	vols []Disk,
) (string, error) {
	dom := Domain{}
	err := yaml.Unmarshal([]byte(libvirtVM), &dom)
	if err != nil {
		return "", err
	}

	dom.Name = &id
	dom.Disk = vols
	dom.Memory.Value = ram * 1024 * 1024
	dom.CurrentMemory.Value = ram * 1024 * 1024
	dom.VCpu.Value = vcpus

	dom.Cpu.Topology.Socket = 1
	dom.Cpu.Topology.Thread = 2
	dom.Cpu.Topology.Cores = vcpus / 2

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

	driver := "e1000e"
	driver = "virtio"

	iface, err := lv.network.CreateInterface(driver)
	dom.Interfaces = []Interface{*iface}
	if err != nil {
		return "", err
	}

	xml := dom.ToString()
	fmt.Println(xml)
	result, err := lv.conn.DomainCreateXML(xml, libvirt.DomainStartValidate)
	if err != nil {
		return "", fmt.Errorf("error starting VM: %s", err.Error())
	} else {
		return string(result.Name), nil
	}
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
