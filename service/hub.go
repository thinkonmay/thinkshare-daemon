package service

import (
	"fmt"
	"os/exec"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/api"
	"github.com/thinkonmay/thinkshare-daemon/log"
)

func (daemon *Daemon) HandleWebRTC() {
	for {
		if daemon.SessionToken == "none" {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		session, err := api.GetSessionInfor(daemon.SessionSettingURL, daemon.SessionToken)
		if err != nil {
			log.PushLog("error get session infor : %s\n", err.Error())
			continue
		}

		go func() {
			for {
				if session.Done {
					return
				}

				path, err := FindProcessPath("hub/bin", "hub.exe")
				if err != nil {
					panic(err)
				}
				process := exec.Command(path,
					"--hid", fmt.Sprintf("localhost:%d", daemon.HIDport),
					"--token", session.Token,
					"--grpc", session.GrpcConf,
					"--webrtc", session.WebRTCConf)

				id := daemon.Childprocess.NewChildProcess(process)

				if id != -1 {
					session.Ids = append(session.Ids, id)
				} else {
					log.PushLog("child process subsystem shutdown\n")
					return
				}

				daemon.Childprocess.WaitID(id)
				time.Sleep(2 * time.Second)
			}
		}()

		for {
			time.Sleep(time.Second)
			if daemon.SessionToken == "none" {
				for _, id := range session.Ids {
					daemon.Childprocess.CloseID(id)
				}

				session.Done = true
				break
			}
		}
	}
}
