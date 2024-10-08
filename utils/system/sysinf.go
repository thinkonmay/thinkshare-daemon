package system

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/jaypipes/ghw"
	"github.com/pion/stun"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
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
func GetPrivateIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String(), nil
}

func GetPublicIPCurl() (result string, err error) {
	result = ""
	if result == "" {
		result = strings.Split(getPublicIPCurl("https://ipv4.icanhazip.com/"), "\n")[0]
	}
	if result == "" {
		result = strings.Split(getPublicIPCurl("https://ipv4.icanhazip.com/"), "\n")[0]
	}
	if result == "" {
		result = strings.Split(getPublicIPCurl("https://ipv4.icanhazip.com/"), "\n")[0]
	}
	if result == "" {
		result = getPublicIPSTUN()
	}
	if result == "" {
		result = getPublicIPSTUN()
	}
	if result == "" {
		return "", fmt.Errorf("worker is not connected to internet")
	} else {
		return result, nil
	}
}
func getPublicIPCurl(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		log.PushLog(err.Error())
		return ""
	}

	ip := make([]byte, 1000)
	size, err := resp.Body.Read(ip)
	if err != nil {
		log.PushLog(err.Error())
		return ""
	}

	return string(ip[:size])
}

func getPublicIPSTUN() (result string) {
	addr := "stun.l.google.com:19302"

	// we only try the first address, so restrict ourselves to IPv4
	c, err := stun.Dial("udp4", addr)
	if err != nil {
		log.PushLog("error dial %s", err)
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
		log.PushLog("failed do %s", err)
	}
	if err := c.Close(); err != nil {
		log.PushLog(err.Error())
	}

	return result
}

func GetInfor() (*packet.WorkerInfor, error) {
	hostStat, err := host.Info()
	if err != nil {
		return nil, err
	}
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	gpu, err := ghw.GPU()
	if err != nil {
		return nil, err
	}
	bios, err := ghw.BIOS()
	if err != nil {
		return nil, err
	}
	cpus, err := ghw.CPU()
	if err != nil {
		return nil, err
	}
	stat, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}
	public, err := GetPublicIPCurl()
	if err != nil {
		return nil, err
	}
	private, err := GetPrivateIP()
	if err != nil {
		return nil, err
	}
	ret := &packet.WorkerInfor{
		CPU:  cpus.Processors[0].Model,
		RAM:  fmt.Sprintf("%dMb", vmStat.Total/1024/1024),
		BIOS: fmt.Sprintf("%v", bios),
		Disk: &packet.DiskInfo{
			Total:  stat.Total / 1024 / 1024 / 1024,
			Free:   stat.Free / 1024 / 1024 / 1024,
			Used:   stat.Used / 1024 / 1024 / 1024,
			Path:   stat.Path,
			Fstype: stat.Fstype,
		},

		GPUs:     []string{},
		Sessions: []*packet.WorkerSession{},
		Volumes:  []string{},

		// Get preferred outbound ip of this machine
		PublicIP:  &public,
		PrivateIP: &private,
	}

	ret.Hostname = fmt.Sprintf("%s (OS %s) (arch %s) (kernel ver.%s) (platform ver.%s)",
		hostStat.Hostname,
		hostStat.Platform,
		hostStat.KernelArch,
		hostStat.KernelVersion,
		hostStat.PlatformVersion)

	for _, i := range gpu.GraphicsCards {
		ret.GPUs = append(ret.GPUs, i.DeviceInfo.Product.Name)
	}

	return ret, nil
}
