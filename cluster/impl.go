package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/melbahja/goph"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

type ClusterConfigManifest struct {
	Nodes []Node `yaml:"nodes"`
	Local Host   `yaml:"local"`
}
type ClusterConfigImpl struct {
	ClusterConfigManifest
	nodes []*NodeImpl
}

func NewClusterConfig(manifest_path string) (ClusterConfig, error) {
	return &ClusterConfigImpl{}, nil
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

type NodeImpl struct {
	Ip       string `yaml:"ip"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Role     string `yaml:"role"`

	client     *goph.Client
	httpclient *http.Client
	cancel     *context.CancelFunc
	internal   packet.WorkerInfor
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

func NewNode() (Node, error) {
	return &NodeImpl{}, nil
}

func (node *NodeImpl) Query() error {
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
