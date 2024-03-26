package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/melbahja/goph"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/libvirt"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

type Node struct {
	Ip       string `yaml:"ip"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Role     string `yaml:"role"`

	client *goph.Client
}

type ClusterConfig struct {
	Nodes []Node `yaml:"nodes"`
}

var (
	ldisk   = "/home/huyhoang/thinkshare-daemon/vol.qcow2"
	lbinary = "/home/huyhoang/thinkshare-daemon/daemon"
	virt    *libvirt.VirtDaemon
	network libvirt.Network
	models  []libvirt.VMLaunchModel = []libvirt.VMLaunchModel{}
	nodes   []Node                  = []Node{}
)

func update_gpu(daemon *Daemon) {
	gpus, err := virt.ListGPUs()
	if err != nil {
		log.PushLog("failed to query gpus %s", err.Error())
		return
	}

	gs := []string{}
	for _, g := range gpus {
		if g.Active {
			return
		}
		gs = append(gs, g.Capability.Product.Val)
	}
	daemon.info.GPUs = gs
}

func FileTransfer(client *goph.Client, rfile, lfile string, force bool) error {
	out, err := exec.Command("du", lfile).Output()
	if err != nil {
		return err
	}
	lsize := strings.Split(string(out), "\t")[0]

	out, err = client.Run(fmt.Sprintf("du %s", rfile))
	rsize := strings.Split(string(out), "\t")[0]
	if err == nil && force {
		client.Run(fmt.Sprintf("rm -f %s", rfile))
	}
	if err != nil || force {
		err = client.Upload(lfile, rfile)
		if err != nil {
			return err
		}

		out, err := client.Run(fmt.Sprintf("du %s", rfile))
		if err != nil {
			return err
		}

		rsize = strings.Split(string(out), "\t")[0]
	}

	log.PushLog("%s : local file size %s, remote file size %s", rfile, lsize, rsize)
	return nil
}

func SetupNode(node Node) error {
	client, err := goph.New(node.Username, node.Ip, goph.Password(node.Password))
	if err != nil {
		return err
	}

	binary := fmt.Sprintf("/home/%s/thinkshare-daemon/daemon", node.Username)
	disk := fmt.Sprintf("/home/%s/thinkshare-daemon/vol.qcow2", node.Username)

	client.Run(fmt.Sprintf("mkdir /home/%s/thinkshare-daemon", node.Username))

	abs, _ := filepath.Abs(lbinary)
	err = FileTransfer(client, binary, abs, true)
	if err != nil {
		return err
	}

	abs, _ = filepath.Abs(ldisk)
	err = FileTransfer(client, disk, abs, false)
	if err != nil {
		return err
	}

	go func() {
		node.client = client
		if err != nil {
			return
		}

		log.PushLog("start %s on %s", binary, node.Ip)
		client.Run(fmt.Sprintf("chmod 777 %s", binary))
		client.Run(fmt.Sprintf("chmod 777 %s", disk))
		_, err = client.Run(binary)
		if err != nil {
			log.PushLog(err.Error())
		}
	}()
	return nil
}

func HandleVirtdaemon(daemon *Daemon, cluster *ClusterConfig) {
	if cluster != nil {
		for _, node := range cluster.Nodes {
			err := SetupNode(node)
			if err != nil {
				log.PushLog(err.Error())
				continue
			}

			nodes = append(nodes, node)
		}
	}

	var err error
	virt, err = libvirt.NewVirtDaemon()
	if err != nil {
		log.PushLog("failed to connect libvirt %s", err.Error())
		return
	}

	network, err = libvirt.NewLibvirtNetwork("enp0s25")
	if err != nil {
		log.PushLog("failed to start network %s", err.Error())
		return
	}
	defer network.Close()

	for {
		update_gpu(daemon)
		time.Sleep(time.Second * 20)
	}
}

func (daemon *Daemon) DeployVM(g string) (*packet.WorkerInfor, error) {
	var gpu *libvirt.GPU = nil
	gpus, err := virt.ListGPUs()
	if err != nil {
		return nil, err
	}
	for _, candidate := range gpus {
		if candidate.Active || candidate.Capability.Product.Val != g {
			continue
		}

		gpu = &candidate
		break
	}

	if gpu == nil {
		return nil, fmt.Errorf("unable to find available gpu")
	}

	iface, err := network.CreateInterface(libvirt.Virtio)
	if err != nil {
		return nil, err
	}

	chain := libvirt.NewVolume(ldisk)
	err = chain.PushChain(40)
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	model := libvirt.VMLaunchModel{
		ID:            id,
		VCPU:          8,
		RAM:           8,
		GPU:           []libvirt.GPU{*gpu},
		BackingVolume: []libvirt.Volume{chain},
		Interfaces:    []libvirt.Interface{*iface},
		VDriver:       true,
	}

	models = append(models, model)
	dom, err := virt.DeployVM(model)
	if err != nil {
		return nil, err
	}

	for {
		time.Sleep(time.Second)
		addr, err := network.FindDomainIPs(dom)
		if err != nil {
			log.PushLog("VM ip not available %s", err.Error())
			continue
		} else if addr.Ip == nil {
			continue
		}

		client := http.Client{Timeout: time.Second}
		resp, err := client.Get(fmt.Sprintf("http://%s:60000/ping", *addr.Ip))
		if err != nil {
			continue
		} else if resp.StatusCode != 200 {
			continue
		}

		resp, err = client.Get(fmt.Sprintf("http://%s:60000/info", *addr.Ip))
		if err != nil {
			continue
		} else if resp.StatusCode != 200 {
			continue
		}

		b, _ := io.ReadAll(resp.Body)
		inf := packet.WorkerInfor{}
		err = json.Unmarshal(b, &inf)
		if err != nil {
			log.PushLog("failed unmarshal reponse body %s", err.Error())
			continue
		} else if inf.PrivateIP == nil || inf.PublicIP == nil {
			log.PushLog("VM address is null, retry")
			continue
		}

		log.PushLog("deployed a new worker %s", *addr.Ip)
		return &inf, nil
	}

}

func (daemon *Daemon) ShutdownVM(info *packet.WorkerInfor) error {
	removeVM := func(vm libvirt.Domain) {
		virt.DeleteVM(*vm.Name)
		for _, model := range models {
			if model.ID == *vm.Name {
				for _, v := range model.BackingVolume {
					v.PopChain()
				}
			}
		}
	}

	vms, err := virt.ListVMs()
	if err != nil {
		return err
	}

	for _, vm := range vms {
		ip, err := network.FindDomainIPs(vm)
		if err != nil {
			continue
		}

		if vm.Running && *ip.Ip == *info.PrivateIP {
			removeVM(vm)
			return nil
		}
	}

	return fmt.Errorf("vm not found")
}

func InfoBuilder(info *packet.WorkerInfor) {
	for _, session := range info.Sessions {
		if session.Vm == nil {
			continue
		}

		resp, err := http.Get(fmt.Sprintf("http://%s:60000/info", *session.Vm.PrivateIP))
		if err != nil {
			continue
		}

		ss := packet.WorkerInfor{}
		b, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(b, &ss)
		if err != nil {
			continue
		}

		session.Vm = &ss
	}

	for _, session := range nodes {
		resp, err := http.Get(fmt.Sprintf("http://%s:60000/info", session.Ip))
		if err != nil {
			continue
		}

		ss := packet.WorkerInfor{}
		b, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(b, &ss)
		if err != nil {
			continue
		}

		info.Sessions = append(info.Sessions, ss.Sessions...)
	}
}

func HandleSessionForward(daemon *Daemon, ss *packet.WorkerSession, command string) (*packet.WorkerSession, error) {
	for _, session := range daemon.info.Sessions {
		if session.Id == *ss.Target && session.Vm != nil {
			nss := *ss
			nss.Target = nil
			b, _ := json.Marshal(nss)
			resp, err := http.Post(
				fmt.Sprintf("http://%s:60000/%s", *session.Vm.PrivateIP, command),
				"application/json",
				strings.NewReader(string(b)))
			if err != nil {
				log.PushLog("failed to request %s", err.Error())
				continue
			}

			b, _ = io.ReadAll(resp.Body)
			if resp.StatusCode != 200 {
				log.PushLog("failed to request %s", string(b))
				continue
			}

			err = json.Unmarshal(b, &nss)
			if err != nil {
				log.PushLog("failed to request %s", err.Error())
				continue
			}

			return &nss, nil
		}
	}

	return nil, fmt.Errorf("no receiver detected")
}
