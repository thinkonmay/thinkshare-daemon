package main

import (
	"fmt"
	"os"
	"time"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	grpc "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/update"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

func main() {
	proj := os.Getenv("PROJECT")
	if proj == "" {
		proj = "avmvymkexjarplbxwlnj"
	}


	credential.SetupEnv(proj)
	update.Update()


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

	storages := []*packet.StorageStatus{}
	go func() {
		for {
			time.Sleep(time.Second * 15)
			for _, s := range storages {
				grpc.StorageReport(s)
			}
		}
	}()

	dm := daemon.NewDaemon(grpc, func(p *packet.Partition) {
		log.PushLog("registering storage account for drive %s",p.Mountpoint)
		account, err := credential.ReadOrRegisterStorageAccount(proxy_cred, p)
		if err != nil {
			log.PushLog("unable to register storage device %s", err.Error())
			return
		}

		for _, s := range storages {
			if s.Account.Password == account.Password && s.Account.Username == account.Username {
				s.Info = p
				return
			}
		}

		log.PushLog("matching storage account %s",p.Mountpoint)
		err = credential.StorageAccountMatchWorker(account, worker_cred, p)
		if err != nil {
			log.PushLog("unable to register storage device %s", err.Error())
			return
		}

		storages = append(storages, &packet.StorageStatus{
			Account: &packet.Account{
				Password: account.Password,
				Username: account.Username,
			},
			Info: p,
		})
	})
	dm.TerminateAtTheEnd()
	<-dm.Shutdown
}
