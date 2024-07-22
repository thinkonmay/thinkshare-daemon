package daemon

import (
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
	"github.com/thinkonmay/thinkshare-daemon/cluster"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/libvirt"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

var (
	very_quick_client = http.Client{Timeout: time.Second}
	slow_client       = http.Client{Timeout: time.Hour * 24}
	local_queue       = []string{}
	local_queue_mut   = &sync.Mutex{}

	libvirt_available = true
	child_dir         = "./child"
	los               = "./os.qcow2"
	lapp              = "./app.qcow2"
	sidecars          = []string{"lancache", "do-not-delete"}
	models            = []libvirt.VMLaunchModel{}
	mut               = &sync.Mutex{}

	virt    *libvirt.VirtDaemon
	network libvirt.Network
)

func init() {
	exe, _ := os.Executable()
	base_dir, _ := filepath.Abs(filepath.Dir(exe))
	child_dir = fmt.Sprintf("%s/child", base_dir)
	los = fmt.Sprintf("%s/os.qcow2", base_dir)
	lapp = fmt.Sprintf("%s/app.qcow2", base_dir)
}

func (daemon *Daemon) HandleVirtdaemon() func() {
	var err error
	virt, err = libvirt.NewVirtDaemon()
	if err != nil {
		log.PushLog("failed to connect libvirt %s", err.Error())
		libvirt_available = false
		return func() {}
	}

	network, err = libvirt.NewLibvirtNetwork(daemon.cluster.Interface())
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

	return func() {
		network.Close()
	}
}

func (daemon *Daemon) DeployVM(session *packet.WorkerSession, cancel chan bool) (*packet.WorkerInfor, error) {
	if !libvirt_available {
		return nil, fmt.Errorf("libvirt not available")
	} else if session.Vm == nil {
		return nil, fmt.Errorf("VM not specified")
	}

	gpu, err := waitForGPU(cancel)
	if err != nil {
		return nil, err
	}

	iface, err := network.CreateInterface(libvirt.Virtio)
	if err != nil {
		return nil, err
	}

	os := los
	if session.Vm.Volumes != nil && len(session.Vm.Volumes) != 0 {
		os, err = findVolumesInDir(child_dir, session.Vm.Volumes[0])
		if err != nil {
			return nil, err
		}
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
		if time.Now().UnixMilli()-start > 10*60*1000 {
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
		} else if *inf.PrivateIP == "" || *inf.PublicIP == "" {
			log.PushLog("VM address is empty, retry")
			continue
		}

		log.PushLog("deployed a new worker %s", *addr.Ip)
		return &inf, nil
	}

	models = pre
	virt.DeleteVM(model.ID)
	return nil, fmt.Errorf("timeout deploy new VM")
}

func (daemon *Daemon) DeployVMonNode(node cluster.Node, nss *packet.WorkerSession, cancel chan bool) (*packet.WorkerSession, error) {
	if !libvirt_available {
		return nil, fmt.Errorf("libvirt not available")
	}

	log.PushLog("deploying VM on node %s", node.Name())
	b, _ := json.Marshal(nss)

	url, err := node.RequestBaseURL()
	if err != nil {
		return nil, err
	}

	go func() {
		for len(cancel) == 0 {
			time.Sleep(time.Second * 1)
			very_quick_client.Post(
				fmt.Sprintf("%s/_new", url),
				"application/json",
				strings.NewReader(string(b)))
		}
	}()

	resp, err := slow_client.Post(
		fmt.Sprintf("%s/new", url),
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

func (daemon *Daemon) DeployVMwithVolume(nss *packet.WorkerSession, cancel chan bool) (*packet.WorkerSession, *packet.WorkerInfor, error) {
	if !libvirt_available {
		return nil, nil, fmt.Errorf("libvirt not available")
	} else if nss.Vm == nil {
		return nil, nil, fmt.Errorf("VM not specified")
	} else if len(nss.Vm.Volumes) == 0 {
		return nil, nil, fmt.Errorf("empty volume id")
	}

	volume_id := nss.Vm.Volumes[0]
	for _, local := range daemon.WorkerInfor.Volumes {
		if local == volume_id {
			Vm, err := daemon.DeployVM(nss, cancel)
			return nil, Vm, err
		}
	}

	for _, node := range daemon.cluster.Nodes() {
		volumes, err := node.Volumes()
		if err != nil {
			log.PushLog("ignore session fwd on node %s %s", node.Name(), err.Error())
			continue
		}
		for _, remote := range volumes {
			if remote == volume_id {
				session, err := daemon.DeployVMonNode(node, nss, cancel)
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
	for _, vm := range vms {
		if vm.Name == nil {
			continue
		}

		if result, err := network.FindDomainIPs(vm); err != nil {
			found := false
			for _, sidecar := range sidecars {
				if strings.Contains(*vm.Name, sidecar) {
					found = true
					break
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

	vols, err := listVolumesInDir(child_dir)
	if err != nil {
		return err
	}

	in_use, gpuss, available := []string{}, []string{}, []string{}
	for _, volume := range volumemap {
		in_use = append(in_use, volume)
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

func takeGPU() (*libvirt.GPU, bool, error) {
	var gpu *libvirt.GPU = nil
	gpus, err := virt.ListGPUs()
	if err != nil {
		return nil, false, err
	}
	for _, candidate := range gpus {
		if candidate.Active {
			continue
		}

		gpu = &candidate
		break
	}

	if gpu == nil {
		return nil, false, nil
	}

	return gpu, true, nil
}

func waitForGPU(cancel chan bool) (gpu *libvirt.GPU, err error) {
	wid := uuid.New().String()
	local_queue_mut.Lock()
	local_queue = append(local_queue, wid)
	local_queue_mut.Unlock()

	log.PushLog("queued GPU claim request : %s", wid)
	log.PushLog("new GPU claim queue      : %s", strings.Join(local_queue, "->"))
	defer func() {
		replace := []string{}
		local_queue_mut.Lock()
		defer local_queue_mut.Unlock()
		for _, part := range local_queue {
			if part == wid {
				continue
			}

			replace = append(replace, part)
		}
		local_queue = replace
		if err != nil {
			log.PushLog("cancel GPU claim request : %s", wid)
		} else {
			log.PushLog("passed GPU claim request : %s", wid)
		}
		log.PushLog("new GPU claim queue      : %s", strings.Join(local_queue, "->"))
	}()

	for {
		local_queue_mut.Lock()
		ur_turn := local_queue[0] == wid
		local_queue_mut.Unlock()
		if ur_turn {
			break
		}

		time.Sleep(3 * time.Second)
		if len(cancel) > 0 {
			return nil, fmt.Errorf("deployment canceled")
		}
	}

	for {
		_gpu, found, er := takeGPU()
		if er != nil {
			return nil, er
		} else if !found {
			time.Sleep(time.Second)
		} else {
			gpu = _gpu
			break
		}
	}
	return
}

func listVolumesInDir(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to query volumes %s", err.Error())
	}

	vols := []string{}
	for _, f := range files {
		if f.IsDir() {
			if subdirs, err := listVolumesInDir(fmt.Sprintf("%s/%s", dir, f.Name())); err == nil {
				vols = append(vols, subdirs...)
			} else {
				return nil, err
			}
		}
		if !strings.Contains(f.Name(), "qcow2") {
			continue
		}

		volume_id := strings.Split(filepath.Base(f.Name()), ".qcow2")[0]

		if uuid.Validate(volume_id) != nil {
			continue
		}

		vols = append(vols, volume_id)
	}
	return vols, nil
}

func findVolumesInDir(dir, vol_id string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("failed to query volumes %s", err.Error())
	}

	for _, f := range files {
		if f.IsDir() {
			if subdirs, err := findVolumesInDir(fmt.Sprintf("%s/%s", dir, f.Name()), vol_id); err == nil {
				return subdirs, nil
			}
		}
		if !strings.Contains(f.Name(), "qcow2") {
			continue
		}

		volume_id := strings.Split(filepath.Base(f.Name()), ".qcow2")[0]
		if uuid.Validate(volume_id) != nil {
			continue
		} else if volume_id == vol_id {
			return fmt.Sprintf("%s/%s.qcow2", dir, volume_id), nil
		}

	}

	return "", fmt.Errorf("volume not found")
}
