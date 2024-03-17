package libvirt

const (
	driver_virtio = "virtio"
)

type Network interface {
	CreateInterface(driver string) (*Interface,error)
	Close()
}