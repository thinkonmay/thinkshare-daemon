package cluster

import (
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

type NodeImpl struct {
	NodeManifest

	active       bool
	connectError error

	httpclient *http.Client
	internal   packet.WorkerInfor
}

func NewNode(manifest NodeManifest) (*NodeImpl, error) {
	impl := &NodeImpl{
		NodeManifest: manifest,
		active:       true,
	}

	if err := impl.Query(); err != nil {
		if err := impl.setupNode(); err != nil {
			impl.active = false
			return nil, err
		}
	} else {
		return impl, nil
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
func (node *NodeImpl) fileTransfer(client *goph.Client, rfile, lfile string, force bool) error {
	out, err := exec.Command("du", lfile).Output()
	if err != nil {
		return fmt.Errorf("failed to retrieve file info %s", err.Error())
	}

	lsize := strings.Split(string(out), "\t")[0]
	out, err = client.Run(fmt.Sprintf("du %s", rfile))
	rsize := strings.Split(string(out), "\t")[0]
	if err == nil && force {
		client.Run(fmt.Sprintf("rm -f %s", rfile))
	}
	if err != nil || force {
		_, err := exec.Command("sshpass",
			"-p", node.Password,
			"scp", lfile, fmt.Sprintf("%s@%s:%s", node.Username, node.Ip, rfile),
		).Output()
		if err != nil {
			return err
		}

		out, err := client.Run(fmt.Sprintf("du %s", rfile))
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
		return goph.NewUnknown(node.Username, node.Ip, goph.Password(node.Password))
	}

	client, err := allocate()
	if err != nil {
		return err
	}

	client.Run(fmt.Sprintf("mkdir -p %s", child_dir))

	err = node.fileTransfer(client, lbinary, lbinary, true)
	if err != nil {
		return err
	}

	err = node.fileTransfer(client, lapp, lapp, true)
	if err != nil {
		return err
	}

	err = node.fileTransfer(client, los, los, false)
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
