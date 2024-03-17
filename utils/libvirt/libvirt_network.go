package libvirt

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/digitalocean/go-libvirt"
	"github.com/digitalocean/go-libvirt/socket/dialers"
)

const (
	network_name = "vlnet"
	bridge_name  = "vlbr"
)

func newNetwork(card string) string {
	return fmt.Sprintf(`
	<network>
		<name>%son%s</name>
		<forward dev="%s" mode="nat">
			<interface dev="%s"/>
			<nat>
				<port start="1024" end="65535"/>
			</nat>
		</forward>
		<bridge name="%son%s" stp="on" delay="0"/>
		<domain name="network"/>
		<ip address="192.168.100.1" netmask="255.255.255.0">
			<dhcp>
				<range start="192.168.100.1" end="192.168.100.254"/>
			</dhcp>
		</ip>
	</network>
	`, network_name, card,
		card,
		card,
		bridge_name, card)
}

type LibvirtNetwork struct {
	conn *libvirt.Libvirt
}

func isIPv4(address string) bool {
	return strings.Count(address, ":") < 2
}

func NewLibvirtNetwork() (Network, error) {
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
	if len(nets) > 0 {
		return ret, nil
	}

	iface := ""
	ifis, _ := net.Interfaces()
	for _, i2 := range ifis {
		if !strings.Contains(i2.Flags.String(), "running") ||
			strings.Contains(i2.Flags.String(), "loopback") ||
			strings.Contains(i2.Name, "br") ||
			strings.Contains(i2.Name, "ovs") ||
			strings.Contains(i2.Name, "vnet") {
			continue
		}

		addr, _ := i2.Addrs()
		for _, a := range addr {
			if !isIPv4(a.String()) {
				continue
			}

			iface = i2.Name
		}
	}

	if iface == "" {
		return nil, fmt.Errorf("no network interface was found")
	}

	_, err = ret.conn.NetworkCreateXML(newNetwork(iface))
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (ovs *LibvirtNetwork) Close(){

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

func (network *LibvirtNetwork) FindDomainIPs(dom Domain) []string {
	ips := []string{}
	macs := []string{}
	for _, i2 := range dom.Interfaces {
		macs = append(macs, *i2.Mac.Address)
	}

	database, err := network.getIPMac()
	if err != nil {
		return []string{}
	}

	for k, v := range database {
		for _, v2 := range macs {
			if v2 == k {
				ips = append(ips, v)
			}
		}
	}

	return ips
}
