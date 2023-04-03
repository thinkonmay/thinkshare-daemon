package main

import (
	"fmt"
	"os"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	grpc "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC"
)


func main() {
	short_task := false

	var err error
	out,worker_account,proxy_account,api_key := "",credential.Account{},credential.Account{},credential.ApiKey{}

	for i,arg := range os.Args[1:]{
		if arg == "proxy" {
			for _,arg1 := range os.Args[i:]{
				if arg1 == "list" {
					proxy_account,err = credential.SetupProxyAccount()
					out = fmt.Sprintf("proxy account : %s",proxy_account.Username)
					short_task = true
				} else if arg1 == "reset" {
					os.Remove(credential.ProxySecretFile)
					proxy_account,err = credential.SetupProxyAccount()
					out = fmt.Sprintf("proxy account generated : %s",proxy_account.Username)
					short_task = true
				}
			}
		}

		if arg == "vendor" {
			for _,arg1 := range os.Args[i:]{
				if arg1 == "keygen" {
					api_key,err = credential.SetupApiKey()
					out = fmt.Sprintf("api key : %s",api_key.Key)
					short_task = true
				} else if arg1 == "workers" {
					api_key,_ = credential.SetupApiKey()
				    out,err = credential.FetchWorker(false,api_key)
					short_task = true
				}
			}
		}
	}


	

	if err != nil {
		fmt.Printf("failed : %s",err.Error())
		fmt.Sprintf("",worker_account,proxy_account,api_key)
		return
	} else if short_task {
		fmt.Println("task result :")
		fmt.Println(out)
		return
	}

	proxy_cred, err := credential.UseProxyAccount()
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


	grpc,err := grpc.InitGRPCClient(
		credential.Secrets.Conductor.Hostname,
		credential.Secrets.Conductor.GrpcPort,
		worker_cred)
	if err != nil {
		fmt.Printf("failed to setup grpc: %s", err.Error())
		return
	}

	dm := daemon.NewDaemon(grpc)
	dm.TerminateAtTheEnd()
	<-dm.Shutdown
}
