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
	"github.com/thinkonmay/thinkshare-daemon/utils/libvirt"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
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
type Node struct {
	Ip       string `yaml:"ip"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Role     string `yaml:"role"`
}

type internalNode struct {
	Node
	client   *goph.Client
	cancel   *context.CancelFunc
	internal packet.WorkerInfor
}

func (node *internalNode) queryNode() error {
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
func (node *internalNode) fileTransfer(rfile, lfile string, force bool) error {
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

func (node *internalNode) setupNode() error {
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

type ClusterConfigManifest struct {
	Nodes []Node `yaml:"nodes"`
	Local Host   `yaml:"local"`
}

type ClusterConfig interface {
}

type ClusterConfigImpl struct {
}

func NewClusterConfig(manifest_path string) ClusterConfig {
	return ClusterConfigImpl{}
}
