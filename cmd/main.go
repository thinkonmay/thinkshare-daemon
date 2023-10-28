package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	// grpc "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/persistent/websocket"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/turn"
)

var (
	proj 	 = "https://supabase.thinkmay.net"
	anon_key = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ewogICJyb2xlIjogImFub24iLAogICJpc3MiOiAic3VwYWJhc2UiLAogICJpYXQiOiAxNjk0MDE5NjAwLAogICJleHAiOiAxODUxODcyNDAwCn0.EpUhNso-BMFvAJLjYbomIddyFfN--u-zCf0Swj9Ac6E"
)
func init() {
	project := os.Getenv("TM_PROJECT")
	key     := os.Getenv("TM_ANONKEY")
	if project != "" {
		proj = project
	}
	if key != "" {
		anon_key = key
	}
}


func main() {
	virtual_display := os.Getenv("VIRTUAL_DISPLAY") == "TRUE"
	media.ActivateVirtualDriver()
	defer media.DeactivateVirtualDriver()
	if virtual_display {
		proc := media.StartVirtualDisplay()
		defer proc.Kill()
	}

	proxy_cred, err := credential.InputProxyAccount()
	if err != nil {
		fmt.Printf("failed to find proxy account: %s", err.Error())
		return
	}
	fmt.Println("proxy account found, continue")

	if ports,found := os.LookupEnv("BUILTIN_TURN"); found {
		portrange := strings.Split(ports, "-")
		if len(portrange) != 2 {
			fmt.Println("invalid port range")
		} 

		min,err := strconv.ParseInt(portrange[0], 10, 32)
		if err != nil {
			fmt.Println("invalid port range")
			min = 60000
		}
		max,err := strconv.ParseInt(portrange[1], 10, 32)
		if err != nil {
			fmt.Println("invalid port range")
			max = 65535
		}

		turn.Open(proxy_cred, int(min), int(max),)
		defer turn.Close()
    }

	worker_cred, err := credential.SetupWorkerAccount(proxy_cred)
	if err != nil {
		fmt.Printf("failed to setup worker account: %s", err.Error())
		return
	}

	grpc, err := websocket.InitGRPCClient(
		credential.PROJECT,
		credential.API_VERSION,
		credential.ANON_KEY,
		worker_cred)
	if err != nil {
		fmt.Printf("failed to setup grpc: %s", err.Error())
		return
	}

	blacklists := []*packet.Partition{}
	dm := daemon.NewDaemon(grpc, func(p *packet.Partition) {
		for _,s := range blacklists {
			if s.Mountpoint == p.Mountpoint {
				return
			}
		}

		log.PushLog("registering storage account for drive %s",p.Mountpoint)
		_,err,apierr := credential.ReadOrRegisterStorageAccount(worker_cred,p)
		if apierr != nil {
			log.PushLog("unable to register storage %s", apierr.Error())
		} else if err != nil {
			log.PushLog("unable to read or register credential file, %s", err.Error())
			return
		}

		blacklists = append(blacklists, p)
	})
	dm.TerminateAtTheEnd()
	<-dm.Shutdown
}
