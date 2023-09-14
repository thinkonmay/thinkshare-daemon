package main

import (
	"fmt"
	"os"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	grpc "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/turn"
)

var (
	proj 	 = "fkymwagaibfzyfrzcizz"
	anon_key = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImZreW13YWdhaWJmenlmcnpjaXp6Iiwicm9sZSI6ImFub24iLCJpYXQiOjE2OTA0NDQxMzMsImV4cCI6MjAwNjAyMDEzM30.t4L2y24cn8uNyEsy1C8vG0WVT8P7yxqXwkdTRRKiHoo"
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
