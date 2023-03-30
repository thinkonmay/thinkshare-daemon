package system

import (
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/pion/stun"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/thinkonmay/conductor/protocol/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

// SysInfo saves the basic system information
type SysInfo struct {
	Hostname  string   `json:"os"`
	CPU       string   `json:"cpu"`
	RAM       string   `json:"ram"`
	Bios      string   `json:"bios"`
	Gpu       []string `json:"gpus"`
	Disk      []string `json:"disks"`
	Network   []string `json:"networks"`
	IP        string   `json:"ip"`
	PrivateIP string   `json:"privateip"`
}

// Get preferred outbound ip of this machine
func GetPrivateIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.PushLog(err.Error())
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}

func GetPublicIP() string {
	result := ""
	addr := "stun.l.google.com:19302"

	// we only try the first address, so restrict ourselves to IPv4
	c, err := stun.Dial("udp4", addr)
	if err != nil {
		log.PushLog("dial:", err)
	}
	if err = c.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), func(res stun.Event) {
		if res.Error != nil {
			log.PushLog(res.Error.Error())
		}
		var xorAddr stun.XORMappedAddress
		if getErr := xorAddr.GetFrom(res.Message); getErr != nil {
			log.PushLog(getErr.Error())
		}
		result = xorAddr.IP.String()
	}); err != nil {
		log.PushLog("do:", err)
	}
	if err := c.Close(); err != nil {
		log.PushLog(err.Error())
	}

	return result
}

func GetPublicIPCurl() string {

	resp,err := http.Get("https://ifconfig.me/ip")	
	if err != nil {
		log.PushLog(err.Error())
		return ""
	}

	ip := make([]byte,1000)
	size,err := resp.Body.Read(ip)
	if err != nil {
		log.PushLog(err.Error())
		return ""
	}

	return string(ip[:size]) 
}


func GetInfor() (*packet.WorkerInfor,error) {
	hostStat,err := host.Info()
	if err != nil {
		log.PushLog("unable to get information from system: %s", err.Error())
		return nil,err
	}
	vmStat,err := mem.VirtualMemory()
	if err != nil {
		log.PushLog("unable to get information from system: %s", err.Error())
		return nil,err
	}
	gpu, err := ghw.GPU()
	if err != nil {
		log.PushLog("unable to get information from system: %s", err.Error())
		return nil,err
	}
	bios, err := ghw.BIOS()
	if err != nil {
		log.PushLog("unable to get information from system: %s", err.Error())
		return nil,err
	}
	pcies, err := ghw.Block()
	if err != nil {
		log.PushLog("unable to get information from system: %s", err.Error())
		return nil,err
	}
	cpus, err := ghw.CPU()
	if err != nil {
		log.PushLog("unable to get information from system: %s", err.Error())
		return nil,err
	}
	networks, err := ghw.Network()
	if err != nil {
		log.PushLog("unable to get information from system: %s", err.Error())
		return nil,err
	}

	ret := &packet.WorkerInfor{
		CPU:   cpus.Processors[0].Model,
		RAM:   fmt.Sprintf("%dMb", vmStat.Total/1024/1024),
		BIOS:  fmt.Sprintf("%s version %s", bios.Vendor, bios.Version),

		NICs:  make([]string, 0),
		Disks: make([]string, 0),
		GPUs:  make([]string, 0),

		// Get preferred outbound ip of this machine
		PublicIP : GetPublicIPCurl(),
		PrivateIP : GetPrivateIP(),

		Timestamp: time.Now().Format(time.RFC3339),
	}

	if hostStat.VirtualizationSystem == "" {
		ret.Hostname = fmt.Sprintf("Baremetal %s ( OS %s %s) (arch %s)", hostStat.Hostname, hostStat.Platform, hostStat.PlatformVersion, hostStat.KernelArch)
	} else {
		ret.Hostname = fmt.Sprintf("VM %s ( OS %s %s) (arch %s)", hostStat.Hostname, hostStat.Platform, hostStat.PlatformVersion, hostStat.KernelArch)

	}

	for _, i := range gpu.GraphicsCards {
		ret.GPUs = append(ret.GPUs, i.DeviceInfo.Product.Name)
	}
	for _, i := range pcies.Disks {
		ret.Disks = append(ret.Disks, fmt.Sprintf("%s (Size %dGb)", i.Model, i.SizeBytes/1024/1024/1024))
	}
	for _, i := range networks.NICs {
		if i.MacAddress != "" {
			ret.NICs = append(ret.NICs, fmt.Sprintf("%s (MAC Address %s)", i.Name, i.MacAddress))
		} else {
			ret.NICs = append(ret.NICs, i.Name)
		}
	}

	return ret,nil
}
