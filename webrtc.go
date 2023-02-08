package main

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/OnePlay-Internet/daemon-tool/log"
)

func (daemon *Daemon) HandleWebRTC() {
	for {
		if daemon.sessionToken == "none" {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		session, err := getSessionInfor(daemon.sessionSettingURL, daemon.sessionToken)
		if err != nil {
			log.PushLog("error get session infor : %s\n", err.Error())
			continue
		}

		go func() {
			for {
				if session.done {
					return
				}

				path,err := FindProcessPath(nil,"webrtc.exe")
				if(err != nil) {
					panic(err)
				}
				process := exec.Command(path,
					"--hid", fmt.Sprintf("localhost:%d", daemon.HIDport),
					"--token", session.token,
					"--grpc", session.GrpcConf,
					"--webrtc", session.WebRTCConf)

				id := daemon.childprocess.NewChildProcess(process)

				if id != -1 {
					session.ids = append(session.ids, id)
				} else {
					log.PushLog("child process subsystem shutdown\n")
					return
				}

				daemon.childprocess.WaitID(id)
				time.Sleep(2 * time.Second)
			}
		}()

		for {
			time.Sleep(time.Second)
			if daemon.sessionToken == "none" {
				for _, id := range session.ids {
					daemon.childprocess.CloseID(id)
				}

				session.done = true
				break
			}
		}
	}
}
