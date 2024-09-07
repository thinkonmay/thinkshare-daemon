package cluster

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"gopkg.in/yaml.v2"
)

const (
	Httpport = 60000
)

var (
	very_quick_client = http.Client{Timeout: time.Second}
	quick_client      = http.Client{Timeout: 5 * time.Second}
	local_queue       = []string{}
	local_queue_mut   = &sync.Mutex{}

	libvirt_available = true
	base_dir          = "."
	child_dir         = "./child"
	los               = "./os.qcow2"
	lapp              = "./app.qcow2"
	lbinary           = "./daemon"
)

func init() {
	exe, _ := os.Executable()
	base_dir, _ = filepath.Abs(filepath.Dir(exe))
	child_dir = fmt.Sprintf("%s/child", base_dir)
	los = fmt.Sprintf("%s/os.qcow2", base_dir)
	lapp = fmt.Sprintf("%s/app.qcow2", base_dir)
	lbinary = fmt.Sprintf("%s/daemon", base_dir)
}

type ClusterConfigManifest struct {
	Nodes []NodeManifest `yaml:"nodes"`
	Peers []PeerManifest `yaml:"peers"`
	Local Host           `yaml:"local"`
}
type ClusterConfigImpl struct {
	ClusterConfigManifest
	manifest_path string

	nodes []*NodeImpl
	peers []*PeerImpl
	mut   *sync.Mutex
}

func NewClusterConfig(manifest_path string) (ClusterConfig, error) {
	impl := &ClusterConfigImpl{
		nodes: []*NodeImpl{},
		mut:   &sync.Mutex{},
	}

	fetch_content := func() (ClusterConfigManifest, error) {
		content, err := os.ReadFile(manifest_path)
		if err != nil {
			return ClusterConfigManifest{}, err
		}

		manifest := ClusterConfigManifest{}
		err = yaml.Unmarshal(content, &manifest)
		return manifest, err
	}
	sync_nodes := func() error {
		impl.mut.Lock()
		defer func() {
			impl.mut.Unlock()
			if err := recover(); err != nil {
				log.PushLog("panic in sync_nodes thread : %s", debug.Stack())
			}
		}()

		desired := impl.ClusterConfigManifest.Nodes
		current := impl.nodes

		need_create := []NodeManifest{}
		need_remove := []*NodeImpl{}
		for _, manifest := range desired {
			found := false
			for _, node := range current {
				if node.Ip == manifest.Ip {
					found = true
				}
			}

			if !found {
				need_create = append(need_create, manifest)
			}
		}

		for _, manifest := range current {
			found := false
			for _, node := range desired {
				if node.Ip == manifest.Name() {
					found = true
				}
			}

			if !found {
				need_remove = append(need_remove, manifest)
			}
		}

		for _, create := range need_create {
			if node, err := NewNode(create); err != nil {
				log.PushLog("failed to init node %s", err.Error())
			} else {
				log.PushLog("new node successfully added %s", node.Name())
				impl.nodes = append(impl.nodes, node)
			}
		}
		for _, rm := range need_remove {
			if err := rm.Deinit(); err != nil {
				log.PushLog("failed to deinit node %s %s", rm.Name(), err.Error())
			} else {
				log.PushLog("deinited node %s", rm.Name())
			}

			replace := []*NodeImpl{}
			for _, node := range impl.nodes {
				if node.Name() == rm.Name() {
					continue
				}

				replace = append(replace, node)
			}

			impl.nodes = replace
		}

		return nil
	}

	sync_peers := func() error {
		impl.mut.Lock()
		defer impl.mut.Unlock()

		desired := impl.ClusterConfigManifest.Peers
		current := impl.peers

		need_create := []PeerManifest{}
		need_remove := []*PeerImpl{}
		for _, manifest := range desired {
			found := false
			for _, node := range current {
				if node.Ip == manifest.Ip {
					found = true
				}
			}

			if !found {
				need_create = append(need_create, manifest)
			}
		}

		for _, manifest := range current {
			found := false
			for _, node := range desired {
				if node.Ip == manifest.Ip {
					found = true
				}
			}

			if !found {
				need_remove = append(need_remove, manifest)
			}
		}

		for _, create := range need_create {
			if node, err := NewPeer(create); err != nil {
				log.PushLog("failed to init peer %s", err.Error())
			} else {
				log.PushLog("new peer successfully added %s", node.Name())
				impl.peers = append(impl.peers, node)
			}
		}
		for _, rm := range need_remove {
			if err := rm.Deinit(); err != nil {
				log.PushLog("failed to deinit peer %s %s", rm.Name(), err.Error())
			} else {
				log.PushLog("deinited peer %s", rm.Name())
			}

			replace := []*PeerImpl{}
			for _, node := range impl.peers {
				if node.Name() == rm.Name() {
					continue
				}

				replace = append(replace, node)
			}

			impl.peers = replace
		}

		return nil
	}

	err := (error)(nil)
	impl.ClusterConfigManifest, err = fetch_content()
	if err != nil {
		return nil, err
	}

	if err := sync_nodes(); err != nil {
		return nil, err
	}

	if err := sync_peers(); err != nil {
		return nil, err
	}

	go func() {
		for {
			time.Sleep(time.Second)
			if manifest, err := fetch_content(); err != nil {
				log.PushLog("failed to fetch manifest %s", err.Error())
			} else {
				impl.ClusterConfigManifest = manifest
			}

			if err := sync_nodes(); err != nil {
				log.PushLog("failed to sync node %s", err.Error())
			}

			if err := sync_peers(); err != nil {
				log.PushLog("failed to sync peer %s", err.Error())
			}
		}
	}()

	return impl, nil
}

func (impl *ClusterConfigImpl) TurnServer() (TurnConfig, bool) {
	if impl.ClusterConfigManifest.Local.TurnServer == nil {
		return TurnConfig{}, false
	} else {
		return *impl.ClusterConfigManifest.Local.TurnServer, true
	}
}
func (impl *ClusterConfigImpl) DNSserver() string {
	return impl.ClusterConfigManifest.Local.DNS
}
func (impl *ClusterConfigImpl) Log() (ip, id string, exists bool) {
	if impl.ClusterConfigManifest.Local.Log == nil {
		return "", "", false
	} else {
		return impl.ClusterConfigManifest.Local.Log.IP,
			impl.ClusterConfigManifest.Local.Log.ID,
			true
	}
}
func (impl *ClusterConfigImpl) Interface() string {
	return impl.ClusterConfigManifest.Local.Interface
}
func (impl *ClusterConfigImpl) Pocketbase() bool {
	return impl.ClusterConfigManifest.Local.EnablePocketbase
}

func (impl *ClusterConfigImpl) Nodes() (ns []Node) {
	ns = []Node{}
	for _, n := range impl.nodes {
		ns = append(ns, n)
	}
	return
}
func (impl *ClusterConfigImpl) Peers() (ns []Peer) {
	ns = []Peer{}
	for _, n := range impl.peers {
		ns = append(ns, n)
	}
	return
}

func (impl *ClusterConfigImpl) Deinit() {
	for _, node := range impl.nodes {
		node.Deinit()
	}
}
