package cluster

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/melbahja/goph"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
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
func (impl *ClusterConfigImpl) Domain() *string {
	return impl.ClusterConfigManifest.Local.Domain
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

type NodeImpl struct {
	NodeManifest

	active       bool
	connectError error

	client     *goph.Client
	httpclient *http.Client
	internal   packet.WorkerInfor
}

func NewNode(manifest NodeManifest) (*NodeImpl, error) {
	impl := &NodeImpl{
		NodeManifest: manifest,
		active:       true,
	}

	if err := impl.setupNode(); err != nil {
		impl.active = false
		return nil, err
	}

	err := (error)(nil)
	now := func() int64 { return time.Now().Unix() }
	start := now()
	for now()-start < 60 {
		time.Sleep(time.Second)
		if err = impl.Query(); err != nil {
			log.PushLog("failed to query new node %s", err.Error())
		} else {
			return impl, nil
		}
	}

	impl.active = false
	return nil, fmt.Errorf("timeout query new node")
}
func (impl *NodeImpl) Deinit() error {
	impl.active = false
	return nil
}

// GPUs implements Node.
func (node *NodeImpl) GPUs() ([]string, error) {
	if node.connectError != nil {
		return nil, fmt.Errorf("failed to get gpu: connection Error %s", node.connectError)
	}
	return node.internal.GPUs, nil
}

// Name implements Node.
func (node *NodeImpl) Name() string {
	return node.Ip
}

// RequestBaseURL implements Node.
func (node *NodeImpl) RequestBaseURL() (string, error) {
	if node.connectError != nil {
		return "", fmt.Errorf("failed to get base URL : connection Error %s", node.connectError)
	}
	return fmt.Sprintf("http://%s:%d", node.Ip, Httpport), nil
}

// RequestClient implements Node.
func (node *NodeImpl) RequestClient() (*http.Client, error) {
	if node.connectError != nil {
		return nil, fmt.Errorf("failed to request client: connection Error %s", node.connectError)
	}
	return node.httpclient, nil
}

// Sessions implements Node.
func (node *NodeImpl) Sessions() ([]*packet.WorkerSession, error) {
	if node.connectError != nil {
		return nil, fmt.Errorf("failed to get sessions : connection Error %s", node.connectError)
	}
	return node.internal.Sessions, nil
}

// Volumes implements Node.
func (node *NodeImpl) Volumes() ([]string, error) {
	if node.connectError != nil {
		return nil, fmt.Errorf("failed to volumes : connection Error %s", node.connectError)
	}
	return node.internal.Volumes, nil
}

func (node *NodeImpl) Query() (err error) {
	if !node.active {
		return fmt.Errorf("node is not active")
	}

	defer func() { node.connectError = err }()

	resp, err := quick_client.Get(fmt.Sprintf("http://%s:%d/info", node.Ip, Httpport))
	if err != nil {
		return err
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return fmt.Errorf(string(b))
	}

	ss := packet.WorkerInfor{}
	err = json.Unmarshal(b, &ss)
	if err != nil {
		return err
	} else if ss.PrivateIP == nil || ss.PublicIP == nil {
		return fmt.Errorf("nil ip")
	}

	node.internal = ss
	return nil
}
func (node *NodeImpl) fileTransfer(rfile, lfile string, force bool) error {
	out, err := exec.Command("du", lfile).Output()
	if err != nil {
		return fmt.Errorf("failed to retrieve file info %s", err.Error())
	}

	lsize := strings.Split(string(out), "\t")[0]
	out, err = node.client.Run(fmt.Sprintf("du %s", rfile))
	rsize := strings.Split(string(out), "\t")[0]
	if err == nil && force {
		node.client.Run(fmt.Sprintf("rm -f %s", rfile))
	}
	if err != nil || force {
		_, err := exec.Command("sshpass",
			"-p", node.Password,
			"scp", lfile, fmt.Sprintf("%s@%s:%s", node.Username, node.Ip, rfile),
		).Output()
		if err != nil {
			return err
		}

		out, err := node.client.Run(fmt.Sprintf("du %s", rfile))
		if err != nil {
			return err
		}

		rsize = strings.Split(string(out), "\t")[0]
		log.PushLog("node %s overrided %s : local file size %s, remote file size %s", node.Name(), rfile, lsize, rsize)
	} else {
		log.PushLog("node %s compare   %s : local file size %s, remote file size %s", node.Name(), rfile, lsize, rsize)
	}

	return nil
}

func (node *NodeImpl) setupNode() error {
	allocate := func() (*goph.Client, error) {
		return goph.New(node.Username, node.Ip, goph.Password(node.Password))
	}

	client, err := allocate()
	if err != nil {
		return err
	}

	node.client = client
	client.Run(fmt.Sprintf("mkdir -p %s", child_dir))

	err = node.fileTransfer(lbinary, lbinary, true)
	if err != nil {
		return err
	}

	err = node.fileTransfer(lapp, lapp, true)
	if err != nil {
		return err
	}

	err = node.fileTransfer(los, los, false)
	if err != nil {
		return err
	}

	go func() {
		for node.active {
			rclient, err := allocate()
			if err != nil {
				log.PushLog("failed to connect ssh %s", err.Error())
				time.Sleep(time.Second)
				continue
			}

			log.PushLog("start %s on %s", lbinary, node.Ip)
			rclient.Run(fmt.Sprintf("chmod 777 %s", lbinary))
			rclient.Run(fmt.Sprintf("chmod 777 %s", lapp))
			rclient.Run(lbinary)
			time.Sleep(time.Second * 10)
		}
	}()
	return nil
}

type PeerImpl struct {
	PeerManifest

	active       bool
	connectError error

	httpclient *http.Client
	internal   packet.WorkerInfor
}

func NewPeer(manifest PeerManifest) (*PeerImpl, error) {
	impl := &PeerImpl{
		PeerManifest: manifest,
		httpclient:   &http.Client{},
		active:       true,
	}

	err := (error)(nil)
	now := func() int64 { return time.Now().Unix() }
	start := now()
	for now()-start < 60 {
		time.Sleep(time.Second)
		if err = impl.Query(); err != nil {
			log.PushLog("failed to query new peer %s", err.Error())
		} else {
			return impl, nil
		}
	}

	return nil, fmt.Errorf("timeout query new node")
}
func (impl *PeerImpl) Deinit() error {
	impl.active = false
	return nil
}

// GPUs implements Node.
func (node *PeerImpl) Info() (*packet.WorkerInfor, error) {
	if node.connectError != nil {
		return nil, fmt.Errorf("failed to get gpus : connection Error %s", node.connectError)
	}

	return &node.internal, nil
}

// Name implements Node.
func (node *PeerImpl) Name() string {
	return node.Ip
}

// RequestBaseURL implements Node.
func (node *PeerImpl) RequestBaseURL() (string, error) {
	if node.connectError != nil {
		return "", fmt.Errorf("failed to getBaseURL : connection Error %s", node.connectError)
	}
	return fmt.Sprintf("http://%s:%d", node.Ip, Httpport), nil
}

// RequestClient implements Node.
func (node *PeerImpl) RequestClient() (*http.Client, error) {
	if node.connectError != nil {
		return nil, fmt.Errorf("failed to request client: connection Error %s", node.connectError)
	}
	return node.httpclient, nil
}

func (node *PeerImpl) Query() (err error) {
	if !node.active {
		return fmt.Errorf("node is not active")
	}

	defer func() { node.connectError = err }()

	resp, err := quick_client.Get(fmt.Sprintf("http://%s:%d/info", node.Ip, Httpport))
	if err != nil {
		return err
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return fmt.Errorf(string(b))
	}

	ss := packet.WorkerInfor{}
	err = json.Unmarshal(b, &ss)
	if err != nil {
		return err
	} else if ss.PrivateIP == nil || ss.PublicIP == nil {
		return fmt.Errorf("nil ip")
	}

	node.internal = ss
	return nil
}
