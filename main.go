package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/api"
	"github.com/thinkonmay/thinkshare-daemon/log"
	"github.com/thinkonmay/thinkshare-daemon/service"
)



func TerminateAtTheEnd(daemon *service.Daemon) {
	chann := make(chan os.Signal, 10)
	signal.Notify(chann, syscall.SIGTERM, os.Interrupt)
	<-chann

	daemon.Childprocess.CloseAll()
	time.Sleep(100 * time.Millisecond)
	daemon.Shutdown <- true
}

func main() {
	var err error
	domain := "service.thinkmay.net"
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--url" {
			domain = args[i+1]
		}
	}

	daemon := service.NewDaemon(domain)
	DefaultLogHandler(daemon,true,true);




	go TerminateAtTheEnd(daemon)
	if daemon.ServerToken, err = api.GetServerToken(daemon.SessionRegistrationURL); err != nil {
		log.PushLog("unable to get server token :%s\n", err.Error())
		return
	}

	daemon.HIDport = daemon.HandleDevSim()
	go daemon.HandleWebRTC()

	go func() {
		for {
			if token, err := api.GetSessionToken(daemon.SessionRegistrationURL, daemon.ServerToken); err == nil {
				daemon.SessionToken = token
			} else {
				log.PushLog("unable to get session token :%s\n", err.Error())
			}

			time.Sleep(time.Second)
		}
	}()

	<-daemon.Shutdown
}
