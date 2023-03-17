package main

import (
	"os"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

type Address struct {
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
}

func main() {
	domain := os.Getenv("THINKREMOTE_SUBSYSTEM_URL")
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--url" {
			domain = args[i+1]
		} else if arg == "--auth" {
			credential.SetupProxyAccount(Address{
				PublicIP:  system.GetPublicIP(),
				PrivateIP: system.GetPrivateIP(),
			})
		}
	}

	if domain == "" {
		domain = "service.thinkmay.net"
	}

	dm := daemon.NewDaemon(domain)
	// err := dm.GetServerToken(info)
	// if err != nil {
	// 	panic(err)
	// }

	dm.DefaultLogHandler(true, true)
	dm.HandleDevSim()
	// dm.HandleWebRTC()
	dm.TerminateAtTheEnd()
	<-dm.Shutdown
}
