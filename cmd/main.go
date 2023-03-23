package main

import (
	"fmt"
	"os"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	grpc "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

func main() {
	authonly := false
	address := credential.Address{
		PublicIP:  system.GetPublicIP(),
		PrivateIP: system.GetPrivateIP(),
	}
	for _, arg := range os.Args[1:]{
		if arg == "--auth" {
			os.Remove("./cache.secret.json")
			authonly = true
		}
	}

	proxy_cred, err := credential.SetupProxyAccount(address)
	if err != nil {
		fmt.Printf("failed to setup proxy account: %s", err.Error())
		return
	}

	if authonly {
		return
	}

	fmt.Printf("proxy account found, continue")
	worker_cred, err := credential.SetupWorkerAccount("http://localhost:54321/functions/v1/", address, *proxy_cred)
	if err != nil {
		fmt.Printf("failed to setup worker account: %s", err.Error())
		return
	}


	grpc,err := grpc.InitGRPCClient("localhost",5000,*worker_cred)
	if err != nil {
		fmt.Printf("failed to setup grpc: %s", err.Error())
		return
	}

	// TODO
	dm := daemon.NewDaemon(grpc)
	dm.TerminateAtTheEnd()
	<-dm.Shutdown
}
