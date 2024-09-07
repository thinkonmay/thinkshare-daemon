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
	Interface        string      `yaml:"interface"`
	DNS              string      `yaml:"dns"`
	TurnServer       *TurnConfig `yaml:"turn"`
	EnablePocketbase bool        `yaml:"pocketbase"`
	Log              *struct {
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
	MinPort  int    `json:"min"`
	MaxPort  int    `json:"max"`
	Port     int    `json:"port"`
	PublicIP string `json:"publicip"`
	Backup   string `json:"backup"`
}

type ClusterConfig interface {
	TurnServer() (conf TurnConfig, exist bool)
	Interface() string
	DNSserver() string
	Log() (ip, id string, exists bool)
	Pocketbase() bool

	Nodes() []Node
	Peers() []Peer
	Deinit()
}
