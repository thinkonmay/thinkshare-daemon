package main

import (
	"fmt"
	"os"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	grpc "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
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
	media.ActivateVirtualDriver()
	defer media.DeactivateVirtualDriver()

	credential.SetupEnv(proj,anon_key)
	proxy_cred, err := credential.InputProxyAccount()
	if err != nil {
		fmt.Printf("failed to find proxy account: %s", err.Error())
		return
	}

	if os.Getenv("BUILTIN_TURN") == "TRUE" {
		turn_server := turn.SetupTurn()
		defer turn_server.CloseTurn()
    }

	fmt.Println("proxy account found, continue")
	worker_cred, err := credential.SetupWorkerAccount(proxy_cred)
	if err != nil {
		fmt.Printf("failed to setup worker account: %s", err.Error())
		return
	}

	grpc, err := grpc.InitGRPCClient(
		credential.Secrets.Conductor.Hostname,
		credential.Secrets.Conductor.GrpcPort,
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
		_,err,apierr := credential.ReadOrRegisterStorageAccount(proxy_cred,worker_cred, p)
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
