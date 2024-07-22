package cluster

import (
	"net/http"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

type Host struct {
	Interface string  `yaml:"interface"`
	Domain    *string `yaml:"domain"`
}
type Node interface {
	Name() string
	RequestClient() (*http.Client, error)
	RequestBaseURL() (string, error)
	Volumes() ([]string, error)
	GPUs() ([]string, error)
	Sessions() ([]*packet.WorkerSession, error)
	Query() error
}

type Peer interface {
	Name() string
	RequestClient() (*http.Client, error)
	RequestBaseURL() (string, error)
	Volumes() ([]string, error)
	GPUs() ([]string, error)
	Sessions() ([]*packet.WorkerSession, error)
	Query() error
}

type ClusterConfig interface {
	Interface() string
	Domain() *string
	Nodes() []Node
	Peers() []Peer
	Deinit()
}
