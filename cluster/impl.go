package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	slow_client       = http.Client{Timeout: time.Hour * 24}
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
	Local Host           `yaml:"local"`
}
type ClusterConfigImpl struct {
	ClusterConfigManifest
	manifest_path string

	nodes []*NodeImpl
	mut   *sync.Mutex
}

func NewClusterConfig(manifest_path string) (ClusterConfig, error) {
	impl := &ClusterConfigImpl{
		nodes: []*NodeImpl{},
	}

	fetch_content := func() (ClusterConfigManifest, error) {
		content, err := os.ReadFile(manifest_path)
		if err != nil {
			return ClusterConfigManifest{}, err
		}

		manifest := ClusterConfigManifest{}
		err = yaml.Unmarshal(content, manifest)
		return manifest, err
	}

	sync_nodes := func() error {
		impl.mut.Lock()
		defer impl.mut.Unlock()

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
				if node.Ip == manifest.Ip {
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
				impl.nodes = append(impl.nodes, node)
			}
		}
		for _, rm := range need_remove {
			if err := rm.Deinit(); err != nil {
				log.PushLog("failed to deinit node %s", err.Error())
			}

		}

		return nil
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
		}
	}()

	manifest, err := fetch_content()
	if err != nil {
		return nil, err
	}

	impl.ClusterConfigManifest = manifest
	return impl, nil
}

func (impl *ClusterConfigImpl) Interface() string {
	return impl.ClusterConfigManifest.Local.Interface
}
func (impl *ClusterConfigImpl) Nodes() (ns []Node) {
	ns = []Node{}
	for _, n := range impl.nodes {
		ns = append(ns, n)
	}
	return
}
func (impl *ClusterConfigImpl) Deinit() {
	for _, node := range impl.nodes {
		cancel := *node.cancel
		cancel()
	}
}

type NodeManifest struct {
	Ip       string `yaml:"ip"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Role     string `yaml:"role"`
}

type NodeImpl struct {
	NodeManifest

	active bool

	client     *goph.Client
	httpclient *http.Client
	cancel     *context.CancelFunc
	internal   packet.WorkerInfor
}

func NewNode(manifest NodeManifest) (*NodeImpl, error) {
	impl := &NodeImpl{
		NodeManifest: manifest,
		active:       true,
	}

	if err := impl.setupNode(); err != nil {
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
			log.PushLog("new node successfully")
			return impl, nil
		}
	}

	return nil, fmt.Errorf("timeout query new node")
}
func (impl *NodeImpl) Deinit() error {
	(*impl.cancel)()
	impl.active = false
	return nil
}

// GPUs implements Node.
func (node *NodeImpl) GPUs() []string {
	return node.internal.GPUs
}

// Name implements Node.
func (node *NodeImpl) Name() string {
	return node.Ip
}

// RequestBaseURL implements Node.
func (node *NodeImpl) RequestBaseURL() string {
	return fmt.Sprintf("http://%s:%d", node.Ip, Httpport)
}

// RequestClient implements Node.
func (node *NodeImpl) RequestClient() *http.Client {
	return node.httpclient
}

// Sessions implements Node.
func (node *NodeImpl) Sessions() []*packet.WorkerSession {
	return node.internal.Sessions
}

// Volumes implements Node.
func (node *NodeImpl) Volumes() []string {
	return node.internal.Volumes
}

func (node *NodeImpl) Query() error {
	if !node.active {
		return fmt.Errorf("node is not active")
	}

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
	}

	log.PushLog("%s : local file size %s, remote file size %s", rfile, lsize, rsize)
	return nil
}

func (node *NodeImpl) setupNode() error {
	client, err := goph.New(node.Username, node.Ip, goph.Password(node.Password))
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
		for {
			client.Conn.Wait()

			time.Sleep(time.Second)
			for {
				client, err = goph.New(node.Username, node.Ip, goph.Password(node.Password))
				if err != nil {
					time.Sleep(time.Second)
					log.PushLog("failed to connect ssh to node %s", err.Error())
					continue
				}

				node.client = client
				break
			}
		}
	}()

	go func() {
		for {
			if client == nil {
				log.PushLog("ssh client is nil, wait for 1 second")
				time.Sleep(time.Second)
				continue
			}

			log.PushLog("start %s on %s", lbinary, node.Ip)
			client.Run(fmt.Sprintf("chmod 777 %s", lbinary))
			client.Run(fmt.Sprintf("chmod 777 %s", lapp))

			var ctx context.Context
			ctx, cancel := context.WithCancel(context.Background())
			node.cancel = &cancel
			_, err = client.RunContext(ctx, lbinary)
			if err != nil {
				log.PushLog(err.Error())
			}

			time.Sleep(time.Second * 10)
		}
	}()
	return nil
}
