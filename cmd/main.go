package main

import (
	"os"
	"github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)




func main() {
	domain := os.Getenv("THINKREMOTE_SUBSYSTEM_URL");
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--url" {
			domain = args[i+1]
		}
	}

	if domain == "" {
		domain = "service.thinkmay.net"
	}

	dm := daemon.NewDaemon(domain)
	err := dm.GetServerToken(system.GetInfor());
	if err != nil  {
		panic(err);
	}

	dm.DefaultLogHandler(true,true);
	dm.HandleDevSim()
	dm.HandleWebRTC()
	dm.TerminateAtTheEnd()
	<-dm.Shutdown
}
