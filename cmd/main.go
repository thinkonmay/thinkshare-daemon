package main

import (
	"fmt"
	"os"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	grpc "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)


const (
	worker_register_url = "https://kczvtfaouddunjtxcemk.functions.supabase.co/worker_register"
	anon_key 			= "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImtjenZ0ZmFvdWRkdW5qdHhjZW1rIiwicm9sZSI6ImFub24iLCJpYXQiOjE2Nzk1NDc0MTcsImV4cCI6MTk5NTEyMzQxN30.dJqF_ipAx8NF_P__tsR-KkghVSc2McQo8B3MxeEup58"
	conductor_domain    = "conductor.thinkmay.net"
	conductor_port      = 5000
)

func main() {
	authonly := false
	address := credential.Address{
		PublicIP:  system.GetPublicIPCurl(),
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

	fmt.Println("proxy account found, continue")
	worker_cred, err := credential.SetupWorkerAccount(address, proxy_cred)
	if err != nil {
		fmt.Printf("failed to setup worker account: %s", err.Error())
		return
	}


	grpc,err := grpc.InitGRPCClient(conductor_domain,conductor_port,*worker_cred)
	if err != nil {
		fmt.Printf("failed to setup grpc: %s", err.Error())
		return
	}

	// TODO
	dm := daemon.NewDaemon(grpc)
	dm.TerminateAtTheEnd()
	<-dm.Shutdown
}
