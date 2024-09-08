package libvirt

const (
	Virtio = "virtio"
)

type DomainAddress struct {
	Mac *string `json:"mac"`
	Ip  *string `json:"ip"`
}

type Network interface {
	FindDomainIPs(dom Domain) (DomainAddress, error)
	CreateInterface(driver string) (*Interface, error)
	Close()
}