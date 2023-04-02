package main

import (
	"fmt"
	"os"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	grpc "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC"
)


func main() {
	authonly := false

	for _, arg := range os.Args[1:]{
		if arg == "--auth" {
			os.Remove(credential.SecretFile)
			authonly = true
		}
	}

	proxy_cred, err := credential.SetupProxyAccount()
	if err != nil {
		fmt.Printf("failed to setup proxy account: %s", err.Error())
		return
	}

	if authonly {
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
