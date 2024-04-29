package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/melbahja/goph"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/libvirt"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

type Host struct {
	Interface string `yaml:"interface"`
}
type Node struct {
	Ip       string `yaml:"ip"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Role     string `yaml:"role"`

	client *goph.Client
	cancel *context.CancelFunc

	internal packet.WorkerInfor
}

type ClusterConfig struct {
	Nodes []Node `yaml:"nodes"`
	Local Host   `yaml:"local"`
}

var (
	very_quick_client = http.Client{Timeout: time.Second}
	quick_client      = http.Client{Timeout: 5 * time.Second}
	slow_client       = http.Client{Timeout: time.Minute * 3}

	libvirt_available = true
	dir               = "."
	child             = "./child"
	los               = "./os.qcow2"
	lapp              = "./app.qcow2"
	lbinary           = "./daemon"
	sidecars          = []string{"lancache", "do-not-delete"}
	models            = []libvirt.VMLaunchModel{}
	nodes             = []*Node{}
	mut               = &sync.Mutex{}

	virt    *libvirt.VirtDaemon
	network libvirt.Network
)

func init() {
	exe, _ := os.Executable()
	dir, _ = filepath.Abs(filepath.Dir(exe))
	child = fmt.Sprintf("%s/child", dir)
	los = fmt.Sprintf("%s/os.qcow2", dir)
	lapp = fmt.Sprintf("%s/app.qcow2", dir)
	lbinary = fmt.Sprintf("%s/daemon", dir)
}

func (daemon *Daemon) HandleVirtdaemon(cluster *ClusterConfig) func() {
	var err error
	virt, err = libvirt.NewVirtDaemon()
	if err != nil {
		log.PushLog("failed to connect libvirt %s", err.Error())
		libvirt_available = false
		return func() {}
	}

	network, err = libvirt.NewLibvirtNetwork(cluster.Local.Interface)
	if err != nil {
		log.PushLog("failed to start network %s", err.Error())
		return func() {}
	}

	if vms, err := virt.ListVMs(); err == nil {
		for _, vm := range vms {
			found := false
			for _, sidecar := range sidecars {
				if sidecar == *vm.Name {
					found = true
				}
			}
			if !found && uuid.Validate(*vm.Name) == nil {
				virt.DeleteVM(*vm.Name)
			}
		}
	}

	if cluster != nil {
		for _, node := range cluster.Nodes {
			err := setupNode(&node)
			if err != nil {
				log.PushLog(err.Error())
				continue
			}

			nodes = append(nodes, &node)
		}
	}

	return func() {
		network.Close()
	}
}

func (daemon *Daemon) DeployVM(session *packet.WorkerSession) (*packet.WorkerInfor, error) {
	if !libvirt_available {
		return nil, fmt.Errorf("libvirt not available")
	} else if session.Vm == nil {
		return nil, fmt.Errorf("VM not specified")
	}

	var gpu *libvirt.GPU = nil
	gpus, err := virt.ListGPUs()
	if err != nil {
		return nil, err
	}
	for _, candidate := range gpus {
		if candidate.Active {
			continue
		}

		gpu = &candidate
		break
	}

	if gpu == nil {
		return nil, fmt.Errorf("ran out of gpu")
	}

	iface, err := network.CreateInterface(libvirt.Virtio)
	if err != nil {
		return nil, err
	}

	os := los
	if session.Vm.Volumes != nil && len(session.Vm.Volumes) != 0 {
		os = fmt.Sprintf("%s/%s.qcow2", child, session.Vm.Volumes[0])
	}

	vcpu := int64(16)
	ram := int64(16)
	if session.Vm.CPU != "" {
		i, err := strconv.ParseInt(session.Vm.CPU, 10, 32)
		if err == nil {
			vcpu = i
		}
	}
	if session.Vm.RAM != "" {
		i, err := strconv.ParseInt(session.Vm.RAM, 10, 32)
		if err == nil {
			ram = i
		}
	}

	disks, err := prepareVolume(os, lapp)
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	model := libvirt.VMLaunchModel{
		ID:            id,
		VCPU:          int(vcpu),
		RAM:           int(ram),
		BackingVolume: disks,
		GPU:           []libvirt.GPU{*gpu},
		Interfaces:    []libvirt.Interface{*iface},
		VDriver:       true,
	}

	pre := make([]libvirt.VMLaunchModel, len(models))
	copy(pre, models)
	models = append(models, model)
	dom, err := virt.DeployVM(model)
	if err != nil {
		for _, d := range disks {
			if d.Disposable || os == los {
				d.PopChain()
			}
		}
		return nil, err
	}

	start := time.Now().UnixMilli()
	for {
		if time.Now().UnixMilli()-start > 3*60*1000 {
			break
		}

		time.Sleep(time.Second)
		addr, err := network.FindDomainIPs(dom)
		if err != nil {
			log.PushLog("VM ip not available %s", err.Error())
			continue
		} else if addr.Ip == nil {
			continue
		}

		resp, err := very_quick_client.Get(fmt.Sprintf("http://%s:%d/ping", *addr.Ip, Httpport))
		if err != nil {
			continue
		} else if resp.StatusCode != 200 {
			continue
		}

		resp, err = very_quick_client.Get(fmt.Sprintf("http://%s:%d/info", *addr.Ip, Httpport))
		if err != nil {
			continue
		} else if resp.StatusCode != 200 {
			continue
		}
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			continue
		}

		if resp.StatusCode != 200 {
			log.PushLog(string(b))
			continue
		}

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

	models = pre
	virt.DeleteVM(model.ID)
	return nil, fmt.Errorf("timeout deploy new VM")
}

func (daemon *Daemon) DeployVMonNode(node Node, nss *packet.WorkerSession) (*packet.WorkerSession, error) {
	if !libvirt_available {
		return nil, fmt.Errorf("libvirt not available")
	}

	log.PushLog("deploying VM on node %s", node.Ip)
	b, _ := json.Marshal(nss)
	resp, err := slow_client.Post(
		fmt.Sprintf("http://%s:%d/new", node.Ip, Httpport),
		"application/json",
		strings.NewReader(string(b)))
	if err != nil {
		return nil, err
	}
	b, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(string(b))
	}

	session := packet.WorkerSession{}
	err = json.Unmarshal(b, &session)
	if err != nil {
		return nil, err
	}

	return &session, nil
}
func (daemon *Daemon) DeployVMonAvailableNode(nss *packet.WorkerSession) (*packet.WorkerSession, error) {
	if !libvirt_available {
		return nil, fmt.Errorf("libvirt not available")
	}

	var node *Node = nil
	for _, n := range nodes {
		resp, err := quick_client.Get(fmt.Sprintf("http://%s:%d/info", n.Ip, Httpport))
		if err != nil {
			continue
		}

		b, err := io.ReadAll(resp.Body)
		if err != nil {
			log.PushLog(err.Error())
			continue
		}
		if resp.StatusCode != 200 {
			log.PushLog("failed to request %s", string(b))
			continue
		}

		info := packet.WorkerInfor{}
		if err != nil {
			log.PushLog(err.Error())
			continue
		}
		err = json.Unmarshal(b, &info)
		if err != nil {
			log.PushLog(err.Error())
			continue
		}

		if len(info.GPUs) > 0 {
			i := *n
			node = &i
			break
		}
	}

	if node == nil {
		return nil, fmt.Errorf("cluster ran out of gpu")
	}

	return daemon.DeployVMonNode(*node, nss)
}

func (daemon *Daemon) DeployVMwithVolume(nss *packet.WorkerSession) (*packet.WorkerSession, *packet.WorkerInfor, error) {
	if !libvirt_available {
		return nil, nil, fmt.Errorf("libvirt not available")
	} else if nss.Vm == nil {
		return nil, nil, fmt.Errorf("VM not specified")
	} else if len(nss.Vm.Volumes) == 0 {
		return nil, nil, fmt.Errorf("empty volume id")
	}

	volume_id := nss.Vm.Volumes[0]
	for _, local := range daemon.info.Volumes {
		if local == volume_id {
			Vm, err := daemon.DeployVM(nss)
			return nil, Vm, err
		}
	}

	for _, node := range nodes {
		for _, remote := range node.internal.Volumes {
			if remote == volume_id {
				session, err := daemon.DeployVMonNode(*node, nss)
				return session, nil, err
			}
		}
	}

	return nil, nil, fmt.Errorf("volume id %s not found", volume_id)
}

func (daemon *Daemon) ShutdownVM(info *packet.WorkerInfor) error {
	if !libvirt_available {
		return fmt.Errorf("libvirt not available")
	}

	removeVM := func(vm libvirt.Domain) {
		if vm.Name == nil {
			return
		}

		virt.DeleteVM(*vm.Name)
		for _, model := range models {
			if model.ID == *vm.Name {
				for _, v := range model.BackingVolume {
					if v.Disposable {
						v.PopChain()
					}
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
		if err != nil || ip.Ip == nil || info.PrivateIP == nil {
			continue
		}

		if vm.Running && *ip.Ip == *info.PrivateIP {
			removeVM(vm)
			return nil
		}
	}

	return fmt.Errorf("vm not found")
}

func (daemon *Daemon) HandleSessionForward(ss *packet.WorkerSession, command string) (*packet.WorkerSession, error) {
	if !libvirt_available {
		return nil, fmt.Errorf("libvirt not available")
	}

	if ss.Target == nil {
		for _, node := range nodes {
			for _, session := range node.internal.Sessions {
				if session.Id != ss.Id {
					continue
				}

				log.PushLog("forwarding command %s to node %s", command, node.Ip)

				b, _ := json.Marshal(ss)
				resp, err := slow_client.Post(
					fmt.Sprintf("http://%s:%d/%s", node.Ip, Httpport, command),
					"application/json",
					strings.NewReader(string(b)))
				if err != nil {
					log.PushLog("failed to request %s", err.Error())
					continue
				}

				b, err = io.ReadAll(resp.Body)
				if err != nil {
					log.PushLog(err.Error())
					continue
				}
				if resp.StatusCode != 200 {
					log.PushLog("failed to request %s", string(b))
					continue
				}

				nss := packet.WorkerSession{}
				err = json.Unmarshal(b, &nss)
				if err != nil {
					log.PushLog("failed to request %s", err.Error())
					continue
				}

				return &nss, nil
			}
		}
		return nil, fmt.Errorf("no session found on any node")
	}

	for _, session := range daemon.info.Sessions {
		if session == nil ||
			ss.Target == nil ||
			session.Id != *ss.Target ||
			session.Vm == nil ||
			session.Vm.PrivateIP == nil {
			continue
		}

		log.PushLog("forwarding command %s to vm %s", command, *session.Vm.PrivateIP)

		nss := *ss
		nss.Target = nil
		b, _ := json.Marshal(nss)
		resp, err := slow_client.Post(
			fmt.Sprintf("http://%s:%d/%s", *session.Vm.PrivateIP, Httpport, command),
			"application/json",
			strings.NewReader(string(b)))
		if err != nil {
			log.PushLog("failed to request %s", err.Error())
			continue
		}

		b, err = io.ReadAll(resp.Body)
		if err != nil {
			log.PushLog("failed to parse request %s", err.Error())
			continue
		}
		if resp.StatusCode != 200 {
			log.PushLog("failed to request %s", string(b))
			continue
		}

		worker_session := packet.WorkerSession{}
		err = json.Unmarshal(b, &worker_session)
		if err != nil {
			log.PushLog("failed to request %s", err.Error())
			continue
		}

		return &worker_session, nil
	}

	for _, node := range nodes {
		for _, session := range node.internal.Sessions {
			if session == nil ||
				session.Id != *ss.Target ||
				session.Vm == nil ||
				session.Vm.PrivateIP == nil {
				continue
			}

			log.PushLog("forwarding command %s to node %s, vm %s", command, node.Ip, *session.Vm.PrivateIP)

			b, _ := json.Marshal(ss)
			resp, err := slow_client.Post(
				fmt.Sprintf("http://%s:%d/%s", node.Ip, Httpport, command),
				"application/json",
				strings.NewReader(string(b)))
			if err != nil {
				log.PushLog("failed to request %s", err.Error())
				continue
			}

			b, err = io.ReadAll(resp.Body)
			if err != nil {
				log.PushLog("failed to parse request %s", err.Error())
				continue
			}
			if resp.StatusCode != 200 {
				log.PushLog("failed to request %s", string(b))
				continue
			}

			nss := packet.WorkerSession{}
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

func (daemon *Daemon) HandleSignaling(token string) (*string, bool) {
	if !libvirt_available {
		return nil, false
	}

	for _, s := range daemon.info.Sessions {
		if s.Id == token && s.Vm != nil {
			return s.Vm.PrivateIP, true
		}
	}
	for _, node := range nodes {
		for _, s := range node.internal.Sessions {
			if s.Id == token && s.Vm != nil {
				return &node.Ip, false
			}

		}

	}
	return nil, false
}

func querySession(session *packet.WorkerSession) error {
	if session == nil ||
		session.Vm == nil ||
		session.Vm.PrivateIP == nil {
		return fmt.Errorf("nil session")
	}

	resp, err := very_quick_client.Get(fmt.Sprintf("http://%s:%d/info", *session.Vm.PrivateIP, Httpport))
	if err != nil {
		return err
	}

	ss := packet.WorkerInfor{}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return fmt.Errorf(string(b))
	}

	err = json.Unmarshal(b, &ss)
	if err != nil {
		return err
	} else if ss.PrivateIP == nil || ss.PublicIP == nil {
		return fmt.Errorf("nil ip")
	}

	session.Vm = &ss
	return nil
}

func queryNode(node *Node) error {
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

func queryLocal(info *packet.WorkerInfor) error {
	ipmap, volumemap := map[string]string{}, map[string]string{}
	vms, err := virt.ListVMs()
	if err != nil {
		return fmt.Errorf("failed to query vms %s", err.Error())
	}
	gpus, err := virt.ListGPUs()
	if err != nil {
		return fmt.Errorf("failed to query gpus %s", err.Error())
	}
	files, err := os.ReadDir(child)
	if err != nil {
		return fmt.Errorf("failed to query volumes %s", err.Error())
	}
	for _, vm := range vms {
		if vm.Name == nil {
			continue
		}

		if result, err := network.FindDomainIPs(vm); err != nil {
			found := false
			for _, sidecar := range sidecars {
				if sidecar == *vm.Name {
					found = true
				}
			}
			if !found {
				log.PushLog("failed to find domain %s ip %s", *vm.Name, err.Error())
			}
		} else if result.Ip == nil {
			log.PushLog("failed to find domain ip, ip is nil")
		} else {
			ipmap[*result.Ip] = *vm.Name
		}
	}
	for _, vm := range vms {
		if vm.Name == nil {
			continue
		}

		var volume_id *string = nil
		for _, model := range models {
			if len(model.BackingVolume) == 0 || model.ID != *vm.Name {
				continue
			}

			splits := strings.Split(filepath.Base(model.BackingVolume[0].Path), ".qcow2")
			if len(splits) == 0 {
				continue
			}

			vol := splits[0]
			volume_id = &vol
			break
		}

		if volume_id != nil {
			vol := *volume_id
			volumemap[*vm.Name] = vol
		}
	}

	in_use, vols, gpuss, available := []string{}, []string{}, []string{}, []string{}
	for _, volume := range volumemap {
		in_use = append(in_use, volume)
	}

	for _, f := range files {
		if f.IsDir() || !strings.Contains(f.Name(), "qcow2") {
			continue
		}

		volume_id := strings.Split(filepath.Base(f.Name()), ".qcow2")[0]

		if uuid.Validate(volume_id) != nil {
			continue
		}

		vols = append(vols, volume_id)
	}

	for _, vol := range vols {
		found := false
		for _, iu := range in_use {
			if iu == vol {
				found = true
			}
		}
		if found {
			continue
		}

		available = append(available, vol)
	}

	for _, g := range gpus {
		if g.Active {
			continue
		}
		gpuss = append(gpuss, g.Capability.Product.Val)
	}

	info.Volumes = available
	info.GPUs = gpuss
	for _, ss := range info.Sessions {
		if ss.Vm == nil {
			continue
		}

		ip := ss.Vm.PrivateIP
		if ip == nil {
			log.PushLog("ip is nil")
			continue
		}

		name, ok := ipmap[*ip]
		if !ok {
			log.PushLog("vm name not found %s", *ip)
			continue
		}

		volume_id, ok := volumemap[name]
		if !ok {
			log.PushLog("volume map not found %v %s", volumemap, name)
			continue
		}

		ss.Vm.Volumes = []string{volume_id}
	}

	return nil
}

func infoBuilder(cp packet.WorkerInfor) packet.WorkerInfor {
	if !libvirt_available {
		return cp
	}

	for _, node := range nodes {
		cp.Sessions = append(cp.Sessions, node.internal.Sessions...)
		cp.GPUs = append(cp.GPUs, node.internal.GPUs...)
		cp.Volumes = append(cp.Volumes, node.internal.Volumes...)
	}

	return cp
}

func QueryInfo(info *packet.WorkerInfor) packet.WorkerInfor {
	mut.Lock()
	defer mut.Unlock()
	if !libvirt_available {
		return *info
	}

	local := make(chan error)
	jobs := []chan error{local}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				local <- fmt.Errorf("panic occurred: %v", err)
			}
		}()
		local <- queryLocal(info)
	}()

	for _, session := range info.Sessions {
		channel := make(chan error)
		jobs = append(jobs, channel)
		go func(s *packet.WorkerSession, c chan error) {
			defer func() {
				if err := recover(); err != nil {
					c <- fmt.Errorf("panic occurred: %v", err)
				}
			}()
			c <- querySession(s)
		}(session, channel)
	}

	for _, node := range nodes {
		channel := make(chan error)
		jobs = append(jobs, channel)
		go func(s *Node, c chan error) {
			defer func() {
				if err := recover(); err != nil {
					c <- fmt.Errorf("panic occurred: %v", err)
				}
			}()
			c <- queryNode(s)
		}(node, channel)
	}

	for _, job := range jobs {
		if err := <-job; err != nil {
			log.PushLog("failed to execute job : %s", err.Error())
		}
	}

	return infoBuilder(*info)
}

func prepareVolume(os, app string) ([]libvirt.Volume, error) {
	chain_app := libvirt.NewVolume(app)
	err := chain_app.PushChain(5)
	if err != nil {
		return []libvirt.Volume{}, err
	}

	result, err := exec.Command("qemu-img", "info", os, "--output", "json").Output()
	if err != nil {
		chain_app.PopChain()
		return []libvirt.Volume{}, fmt.Errorf("failed to retrieve disk info %s", err.Error())
	}

	var chain_os *libvirt.Volume = nil
	result_data := struct {
		Backing *string `json:"backing-filename"`
	}{}
	err = json.Unmarshal(result, &result_data)
	if err != nil {
		chain_app.PopChain()
		return []libvirt.Volume{}, err
	} else if result_data.Backing != nil {
		chain_os = libvirt.NewVolume(os, *result_data.Backing)
	} else {
		chain_os = libvirt.NewVolume(os)
		err = chain_os.PushChain(240)
		if err != nil {
			chain_app.PopChain()
			return []libvirt.Volume{}, err
		}
	}

	chain_os.Disposable = false
	return []libvirt.Volume{*chain_os, *chain_app}, nil
}

func deinit() {
	for _, node := range nodes {
		cancel := *node.cancel
		cancel()
	}
}

func fileTransfer(node *Node, rfile, lfile string, force bool) error {
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

func setupNode(node *Node) error {
	client, err := goph.New(node.Username, node.Ip, goph.Password(node.Password))
	if err != nil {
		return err
	}

	node.client = client
	binary := "~/thinkshare-daemon/daemon"
	app := "~/thinkshare-daemon/app.qcow2"
	os := "~/thinkshare-daemon/os.qcow2"

	client.Run("mkdir ~/thinkshare-daemon")
	client.Run("mkdir ~/thinkshare-daemon/child")

	abs, _ := filepath.Abs(lbinary)
	err = fileTransfer(node, binary, abs, true)
	if err != nil {
		return err
	}

	abs, _ = filepath.Abs(lapp)
	err = fileTransfer(node, app, abs, true)
	if err != nil {
		return err
	}

	abs, _ = filepath.Abs(los)
	err = fileTransfer(node, os, abs, false)
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
			log.PushLog("start %s on %s", binary, node.Ip)
			client.Run(fmt.Sprintf("chmod 777 %s", binary))
			client.Run(fmt.Sprintf("chmod 777 %s", app))

			var ctx context.Context
			ctx, cancel := context.WithCancel(context.Background())
			node.cancel = &cancel
			_, err = client.RunContext(ctx, binary)
			if err != nil {
				log.PushLog(err.Error())
			}

			time.Sleep(time.Second * 10)
		}
	}()
	return nil
}
