package cluster

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

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
