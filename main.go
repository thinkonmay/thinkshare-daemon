package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	childprocess "github.com/OnePlay-Internet/daemon-tool/child-process"
	"github.com/OnePlay-Internet/daemon-tool/log"
)

type Daemon struct {
	logURL                 string
	sessionSettingURL      string
	sessionRegistrationURL string

	serverToken  string
	sessionToken string

	HIDport int

	childprocess *childprocess.ChildProcesses
	shutdown     chan bool
}

func TerminateAtTheEnd(daemon *Daemon) {
	chann := make(chan os.Signal, 10)
	signal.Notify(chann, syscall.SIGTERM, os.Interrupt)
	<-chann

	daemon.childprocess.CloseAll()
	time.Sleep(100 * time.Millisecond)
	daemon.shutdown <- true
}

func FindProcessPath(dir *string,process string) (string,error){
	cmd := exec.Command("where.exe",process)

	if dir != nil {
		cmd.Dir = *dir
	}

	bytes,err := cmd.Output()
	if err != nil{
		return "",nil
	}
	paths := strings.Split(string(bytes), "\n")
	pathss := strings.Split(paths[0], "\r")
	return pathss[0],nil
}

func main() {
	var err error
	domain := "service.dev.thinkmay.net"
	args := os.Args[1:]
	for i, arg := range args {
		if arg == "--url" {
			domain = args[i+1]
		}
	}

	daemon := Daemon{
		shutdown:               make(chan bool),
		serverToken: 			"none",
		sessionToken:           "none",
		sessionRegistrationURL: fmt.Sprintf("https://%s/api/worker", domain),
		sessionSettingURL:      fmt.Sprintf("https://%s/api/session/setting", domain),
		logURL:                 fmt.Sprintf("https://%s/api/log/worker", domain),
		childprocess:           childprocess.NewChildProcessSystem(),
	}

	go func ()  {
		for {
			out := log.TakeLog()
			if daemon.serverToken == "none" {
				continue
			}
			PushLog(daemon.logURL,daemon.serverToken,out)
		}
	}()

	go TerminateAtTheEnd(&daemon)
	if daemon.serverToken, err = getServerToken(daemon.sessionRegistrationURL); err != nil {
		log.PushLog("unable to get server token :%s\n", err.Error())
		return
	}

	daemon.HIDport = daemon.HandleDevSim()
	go func() {
		for {
			if token, err := getSessionToken(daemon.sessionRegistrationURL, daemon.serverToken); err == nil {
				daemon.sessionToken = token
			} else {
				log.PushLog("unable to get session token :%s\n", err.Error())
			}

			time.Sleep(time.Second)
		}
	}()

	go daemon.HandleWebRTC()
	<-daemon.shutdown
}
