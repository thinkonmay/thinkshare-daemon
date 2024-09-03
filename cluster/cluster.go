package cluster

import (
	"net/http"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

type NodeManifest struct {
	Ip       string `yaml:"ip"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}
type PeerManifest struct {
	Ip string `yaml:"ip"`
}
type Host struct {
	Interface  string      `yaml:"interface"`
	DNS        string      `yaml:"dns"`
	TurnServer *TurnConfig `yaml:"turn"`
	Domain     *struct {
		Service    string `yaml:"service"`
		Data       string `yaml:"data"`
		Management string `yaml:"management"`
		Monitoring string `yaml:"monitoring"`
	} `yaml:"domain"`
	Log *struct {
		IP string `yaml:"ip"`
		ID string `yaml:"id"`
	} `yaml:"log"`
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
	Info() (*packet.WorkerInfor, error)
	Query() error
}

type TurnConfig struct {
	MinPort  int    `json:"min_port"`
	MaxPort  int    `json:"max_port"`
	Port     int    `json:"port"`
	PublicIP string `json:"public_ip"`
}

type ClusterConfig interface {
	TurnServer() (conf TurnConfig, exist bool)
	Interface() string
	DNSserver() string
	Log() (ip, id string, exists bool)
	Domain() (service, admin string, ok bool)

	Nodes() []Node
	Peers() []Peer
	Deinit()
}
