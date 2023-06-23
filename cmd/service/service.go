package main

import (
	"fmt"
	"os"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	grpc "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

var (
	proj 	 = "avmvymkexjarplbxwlnj"
	anon_key = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImF2bXZ5bWtleGphcnBsYnh3bG5qIiwicm9sZSI6ImFub24iLCJpYXQiOjE2ODAzMjM0NjgsImV4cCI6MTk5NTg5OTQ2OH0.y2W9svI_4O4_xd5AQk4S4MLJAvQJIp0QrO4cljLB9Ik"
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
	credential.SetupEnv(proj,anon_key)
	proxy_cred, err := credential.InputProxyAccount()
	if err != nil {
		fmt.Printf("failed to find proxy account: %s", err.Error())
		return
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

	storages := []struct{
		Info    packet.Partition  
		Account credential.Account   
	}{}

	dm := daemon.NewDaemon(grpc, func(p *packet.Partition) {
		for _,s := range storages {
			if s.Info.Mountpoint == p.Mountpoint {
				return
			}
		}

		log.PushLog("registering storage account for drive %s",p.Mountpoint)
		account, err := credential.ReadOrRegisterStorageAccount(proxy_cred,worker_cred, p)
		if err != nil {
			log.PushLog("unable to register storage device %s", err.Error())
			return
		}

		storages = append(storages, struct{Info packet.Partition; Account credential.Account}{
			Info: *p,
			Account: *account,
		})
	})
	dm.TerminateAtTheEnd()
	<-dm.Shutdown
}
