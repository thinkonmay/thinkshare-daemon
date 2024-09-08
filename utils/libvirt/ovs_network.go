package libvirt

import (
	"fmt"
	"net"
	"strings"

	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/google/uuid"
)

type OpenVSwitch struct {
	svc *ovs.Client

	bridge string
	ports  []string
}

func NewOVS(iface string) (Network, error) {
	found := false
	ifis, _ := net.Interfaces()
	for _, i2 := range ifis {
		if !strings.Contains(i2.Flags.String(), "running") ||
			strings.Contains(i2.Flags.String(), "loopback") ||
			strings.Contains(i2.Name, "br") ||
			strings.Contains(i2.Name, "ovs") ||
			strings.Contains(i2.Name, "vnet") {
			continue
		}

		if iface == i2.Name {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("no network interface was found")
	}

	svc := ovs.New()
	now := uuid.NewString()
	err := svc.VSwitch.AddBridge(now)
	if err != nil {
		return nil, err
	}
	err = svc.VSwitch.AddPort(now, iface)
	if err != nil {
		return nil, err
	}
	return &OpenVSwitch{
		svc:    svc,
		bridge: now,
		ports:  []string{iface},
	}, nil
}

func (ovs *OpenVSwitch) Close() {
	for _, p := range ovs.ports {
		ovs.svc.VSwitch.DeletePort(ovs.bridge, p)
	}
	ovs.svc.VSwitch.DeleteBridge(ovs.bridge)
}

func (ovs *OpenVSwitch) CreateInterface(driver string) (*Interface, error) {
	now := uuid.NewString()
	err := ovs.svc.VSwitch.AddPort(ovs.bridge, now)
	if err != nil {
		return nil, err
	}

	ovs.ports = append(ovs.ports, now)
	return &Interface{
		Type: "bridge",
		VirtualPort: &struct {
			Type string "xml:\"type,attr\""
		}{
			Type: "openvswitch",
		},
		Source: &InterfaceSource{
			Bridge: &ovs.bridge,
		},
		Target: &struct {
			Dev string "xml:\"dev,attr\""
		}{
			Dev: now,
		},
		Model: &struct {
			Type *string "xml:\"type,attr\""
		}{
			Type: &driver,
		},
	}, nil
}


func (network *OpenVSwitch) FindDomainIPs(dom Domain) (DomainAddress,error) {
	macs := []string{}
	for _, i2 := range dom.Interfaces {
		if i2.Mac == nil {
			continue
		} else if i2.Mac.Address == nil {
			continue
		}

		macs = append(macs, *i2.Mac.Address)
	}
	if len(macs) == 0 {
		return DomainAddress{Mac: nil,Ip: nil},nil
	}
	return DomainAddress{Mac: &macs[0],Ip: nil},nil
}
