package credential

import (
	"fmt"
	"net"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

const (
	SecretDir       = "./secret"
	ProxySecretFile = "./secret/proxy.json"

	API_VERSION = "v1"
	PROJECT     = "supabase.thinkmay.net"
	ANON_KEY    = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ewogICJyb2xlIjogImFub24iLAogICJpc3MiOiAic3VwYWJhc2UiLAogICJpYXQiOiAxNjk0MDE5NjAwLAogICJleHAiOiAxODUxODcyNDAwCn0.EpUhNso-BMFvAJLjYbomIddyFfN--u-zCf0Swj9Ac6E"
)

type Account struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
}

var Addresses = &struct {
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
}{}

func init() {
	for {
		Addresses.PublicIP = system.GetPublicIPCurl()
		Addresses.PrivateIP = system.GetPrivateIP()
		if Addresses.PrivateIP != "" && Addresses.PublicIP != "" {
			break
		}

		log.PushLog("server is not connected to the internet")
		time.Sleep(10 * time.Second)
	}
}

func GetFreeUDPPort(min int, max int) (int, error) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	port := l.LocalAddr().(*net.UDPAddr).Port
	if port > max {
		return 0, fmt.Errorf("invalid port %d", port)
	} else if port < min {
		return GetFreeUDPPort(min, max)
	}
	return port, nil
}
