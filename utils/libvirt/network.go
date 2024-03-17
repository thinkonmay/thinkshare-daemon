package libvirt

const (
	driver_virtio = "virtio"
)

type DomainAddress struct {
	Mac *string `json:"mac"`
	Ip  *string `json:"ip"`
}

type Network interface {
	FindDomainIPs(dom Domain) (DomainAddress,error)
	CreateInterface(driver string) (*Interface,error)
	Close()
}