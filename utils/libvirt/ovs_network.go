package libvirt

import (
	"fmt"
	"net"

	"github.com/digitalocean/go-openvswitch/ovs"
	"github.com/google/uuid"
)

type OpenVSwitch struct {
	svc *ovs.Client

	bridge string
	ports  []string
}

func NewOVS(iface string) (Network, error) {
	ifis, _ := net.Interfaces()
	throw := true
	for _, i2 := range ifis {
		if i2.Name == iface {
			throw = false
		}
	}
	if throw {
		return nil, fmt.Errorf("not iface was found %s", iface)
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
