package cluster

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/libvirt"
)

const (
	Httpport = 60000
)

var (
	very_quick_client = http.Client{Timeout: time.Second}
	quick_client      = http.Client{Timeout: 5 * time.Second}
	slow_client       = http.Client{Timeout: time.Hour * 24}
	local_queue       = []string{}
	local_queue_mut   = &sync.Mutex{}

	libvirt_available = true
	base_dir          = "."
	child_dir         = "./child"
	los               = "./os.qcow2"
	lapp              = "./app.qcow2"
	lbinary           = "./daemon"
	sidecars          = []string{"lancache", "do-not-delete"}
	models            = []libvirt.VMLaunchModel{}
	mut               = &sync.Mutex{}

	virt    *libvirt.VirtDaemon
	network libvirt.Network
)

func init() {
	exe, _ := os.Executable()
	base_dir, _ = filepath.Abs(filepath.Dir(exe))
	child_dir = fmt.Sprintf("%s/child", base_dir)
	los = fmt.Sprintf("%s/os.qcow2", base_dir)
	lapp = fmt.Sprintf("%s/app.qcow2", base_dir)
	lbinary = fmt.Sprintf("%s/daemon", base_dir)
}

type Host struct {
	Interface string `yaml:"interface"`
}
type Node interface {
	Name() string
	RequestClient() *http.Client
	RequestBaseURL() string
	Volumes() []string
	GPUs() []string
	Sessions() []*packet.WorkerSession
	Query() error
}

type ClusterConfig interface {
	Interface() string
	Nodes() []Node
	Deinit()
}
