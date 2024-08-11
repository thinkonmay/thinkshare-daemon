package libvirt

import (
	"fmt"
	"net"
	"strings"

	"math/rand/v2"

	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
)

func newNetwork(card, dns string) string {
	rand := rand.IntN(63) + 1
	ip := fmt.Sprintf("10.10.%d.1", rand)
	endip := fmt.Sprintf("10.10.%d.254", rand)

	return fmt.Sprintf(`
	<network>
		<name>%s</name>
		<forward dev="%s" mode="nat">
			<interface dev="%s"/>
		</forward>
		<bridge name="%sbr" stp="on" delay="0"/>
		<dns>
    		<forwarder addr='%s'/>
  		</dns>
		<ip address="%s" netmask="255.255.255.0">
			<dhcp>
				<range start="%s" end="%s"/>
			</dhcp>
		</ip>
	</network>
	`, card, card, card, card, dns, ip, ip, endip)
}

type LibvirtNetwork struct {
	conn *libvirt.Libvirt
}

func isIPv4(address string) bool {
	return strings.Count(address, ":") < 2
}

func NewLibvirtNetwork(iface, dns string) (Network, error) {
	c, err := net.DialTimeout("unix", "/var/run/libvirt/libvirt-sock", 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to dial libvirt: %v", err)
	}

	ret := &LibvirtNetwork{
		conn: libvirt.NewWithDialer(dialers.NewAlreadyConnected(c)),
	}

	if err := ret.conn.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	nets, _, _ := ret.conn.ConnectListAllNetworks(1, libvirt.ConnectListNetworksActive)
	for _, net := range nets {
		if net.Name == iface {
			return ret, nil
		}
	}

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

	_, err = ret.conn.NetworkCreateXML(newNetwork(iface, dns))
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (ovs *LibvirtNetwork) Close() {
	ovs.conn.Disconnect()

}

func (ovs *LibvirtNetwork) CreateInterface(driver string) (*Interface, error) {
	nets, _, _ := ovs.conn.ConnectListAllNetworks(1, libvirt.ConnectListNetworksActive)

	if len(nets) == 0 {
		return nil, fmt.Errorf("not found any vnet")
	}

	Name := nets[0].Name
	return &Interface{
		Type: "network",
		Source: &InterfaceSource{
			Network: &Name,
		},
		Model: &struct {
			Type *string "xml:\"type,attr\""
		}{
			Type: &driver,
		},
	}, nil
}

func (ovs *LibvirtNetwork) getIPMac() (map[string]string, error) {
	nets, _, err := ovs.conn.ConnectListAllNetworks(1, libvirt.ConnectListNetworksActive)
	if err != nil {
		return map[string]string{}, err
	}

	ipmacs := map[string]string{}
	for _, n := range nets {
		leases, _, err := ovs.conn.NetworkGetDhcpLeases(n, []string{}, 1, 0)
		if err != nil {
			return map[string]string{}, err
		}
		for _, ndl := range leases {
			for _, v := range ndl.Mac {
				ipmacs[v] = ndl.Ipaddr
			}
		}
	}

	return ipmacs, nil
}

func (network *LibvirtNetwork) FindDomainIPs(dom Domain) (DomainAddress, error) {
	macs := []string{}
	for _, i2 := range dom.Interfaces {
		macs = append(macs, *i2.Mac.Address)
	}

	database, err := network.getIPMac()
	if err != nil {
		return DomainAddress{}, err
	}

	for k, v := range database {
		for _, v2 := range macs {
			if v2 == k && isIPv4(v) {
				return DomainAddress{
					Mac: &k,
					Ip:  &v,
				}, nil
			}
		}
	}

	return DomainAddress{Mac: nil, Ip: nil}, fmt.Errorf("not found")
}
