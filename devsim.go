package main

import (
	"fmt"
	"os/exec"
	"time"

	childprocess "github.com/OnePlay-Internet/daemon-tool/child-process"
	"github.com/OnePlay-Internet/daemon-tool/log"
)

type DevSim struct {
	id childprocess.ProcessID
}

func (daemon *Daemon) HandleDevSim() int {
	devsim := &DevSim{}

	hidport := 5000
	done := make(chan bool)
	go func() {
		for {
			path,err := FindProcessPath(nil,"DevSim.exe")
			if(err != nil) {
				panic(err)
			}
			process := exec.Command(path, fmt.Sprintf("--urls=http://localhost:%d", hidport))

			failed := childprocess.NewEvent()
			success := childprocess.NewEvent()

			go func() {
				id := daemon.childprocess.NewChildProcess(process)
				if id != -1 {
					devsim.id = id
				} else {
					log.PushLog("child process subsystem shutdown\n")
					return
				}
				daemon.childprocess.WaitID(devsim.id)
				failed.Raise()
				if !success.IsInvoked() {
					hidport++
				}
			}()
			go func() {
				time.Sleep(2 * time.Second)
				success.Raise()
				if !failed.IsInvoked() {
					done <- true
				}
			}()

			for {
				if failed.IsInvoked() {
					break
				} else {
					time.Sleep(1 * time.Second)
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()
	<-done

	return hidport
}
