package main

import (
	"fmt"
	"os"

	"github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)


func main() {
	// domain := os.Getenv("THINKREMOTE_SUBSYSTEM_URL")
	args := os.Args[1:]
	for _, arg := range args {
		if arg == "--auth" {
			_,err := credential.SetupProxyAccount(credential.Data{
				PublicIP:  system.GetPublicIP(),
				PrivateIP: system.GetPrivateIP(),
			})
			if err != nil {
				fmt.Printf("failed to setup proxy account: %s",err.Error())
			}
			return;
		}
	}
	
	data := credential.Data{
		PublicIP  : system.GetPublicIP(),
		PrivateIP : system.GetPrivateIP(),
	}
	cred,err := credential.SetupProxyAccount(data)
	if err != nil {
		fmt.Printf("failed to setup proxy account: %s",err.Error())
		return
	} else {
		fmt.Printf("proxy account found, continue")
	}

	worker_cred,err := credential.SetupWorkerAccount("http://localhost:54321/functions/v1/",data,*cred)

	dm := daemon.NewDaemon(worker_cred.Username)
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
