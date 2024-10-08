package cluster

import (
	"net/http"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

type NodeManifest struct {
	Ip       string `yaml:"ip"`
	Path     string `yaml:"path"`
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
	Addr string `yaml:"address"`
}

type ClusterConfig interface {
	TurnServer() (conf TurnConfig, exist bool)
	Interface() string
	DNSserver() string
	Pocketbase() bool

	Nodes() []Node
	Peers() []Peer
	Deinit()
}
