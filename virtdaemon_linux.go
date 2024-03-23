package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/libvirt"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

var disk_part = "/disk/HHDa/default.qcow2"

var (
	virt *libvirt.VirtDaemon
	network libvirt.Network
)

func HandleVirtdaemon(daemon *Daemon) {
	var err error
	virt, err := libvirt.NewVirtDaemon()
	if err != nil {
		log.PushLog("failed to create virtdaemon %s", err.Error())
		return
	}

	network, err = libvirt.NewLibvirtNetwork("enp0s25")
	if err != nil {
		log.PushLog("failed to query gpus %s", err.Error())
		return
	}
	defer network.Close()

	update_vms := func() {
		vms, err := virt.ListVMs()
		if err != nil {
			log.PushLog("failed to query gpus %s", err.Error())
			return
		}

		doms := []*packet.WorkerInfor{}
		for _, vm := range vms {
			if !vm.Running {
				continue
			}

			addr, err := network.FindDomainIPs(vm)
			if err != nil {
				log.PushLog("failed to query gpus %s", err.Error())
				continue
			} else if addr.Ip == nil {
				continue
			}

			client := http.Client{Timeout: time.Second}
			resp, err := client.Post(fmt.Sprintf("http://%s:60000/info", *addr.Ip), "application/json", strings.NewReader("{}"))
			if err != nil {
				continue
			} else if resp.StatusCode != 200 {
				continue
			}

			b, _ := io.ReadAll(resp.Body)
			inf := packet.WorkerInfor{}
			err = json.Unmarshal(b, &inf)
			if err != nil {
				log.PushLog("failed to query gpus %s", err.Error())
				continue
			}

			doms = append(doms, &inf)
		}

		daemon.vms = doms
	}


	update_gpu := func() {
		gpus, err := virt.ListGPUs()
		if err != nil {
			log.PushLog("failed to query gpus %s", err.Error())
			return
		}

		daemon.gpus = []string{}
		for _, g := range gpus {
			if g.Active {
				return
			}
			daemon.gpus = append(daemon.gpus, g.Capability.Product.Val)
		}
	}

	for {
		update_vms()
		update_gpu()
		time.Sleep(time.Second * 20)
	}
}

func DeployVM(g string) (*libvirt.VMLaunchModel ,error) {
	var gpu *libvirt.GPU = nil
	gpus, err := virt.ListGPUs()
	if err != nil {
		log.PushLog("failed to query gpus %s", err.Error())
	}
	for _, candidate := range gpus {
		if candidate.Active || candidate.Capability.Product.Val != g {
			continue
		}

		gpu = &candidate
		break
	}

	if gpu == nil {
		return nil,fmt.Errorf("unable to find available gpu")
	}

	iface, err := network.CreateInterface(libvirt.Virtio)
	if err != nil {
		return nil,err
	}

	chain := libvirt.NewVolume(disk_part)
	err = chain.PushChain(40)
	if err != nil {
		return nil,err
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

	dom, err := virt.DeployVM(model)
	if err != nil {
		return nil,err
	}

	for {
		time.Sleep(time.Second)
		addr, err := network.FindDomainIPs(dom)
		if err != nil {
			log.PushLog("failed to query gpus %s", err.Error())
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

		log.PushLog("deployed a new worker %s", *addr.Ip)
		break
	}

	return &model,nil
}


func ShutdownVM(model libvirt.VMLaunchModel) error {
	removeVM := func(vm libvirt.Domain) {
		virt.DeleteVM(*vm.Name)
		if model.ID == *vm.Name {
			for _, v := range model.BackingVolume {
				v.PopChain()
			}
		}
	}

	vms, err := virt.ListVMs()
	if err != nil {
		return err
	}

	for _, vm := range vms {
		if !vm.Running {
			continue
		}

		if *vm.Name == model.ID {
			removeVM(vm)
		}
	}

	return nil
}