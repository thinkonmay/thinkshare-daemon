package daemon

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"sync"

	"github.com/thinkonmay/thinkshare-daemon/childprocess"
	"github.com/thinkonmay/thinkshare-daemon/persistent"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/path"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

type internalWorkerSession struct {
	packet.WorkerSession
	childprocess []childprocess.ProcessID
	
	display *int
}

type Daemon struct {
	childprocess *childprocess.ChildProcesses
	persist      persistent.Persistent

	mutex *sync.Mutex

	session []internalWorkerSession
	log     int
}

type DaemonOption struct {
	Turn *struct {
		Username string `json:"username"`
		Password string `json:"password"`
		MinPort  int    `json:"min_port"`
		MaxPort  int    `json:"max_port"`
		Port     int
	} `json:"turn"`
}

func WebDaemon(persistent persistent.Persistent,
				options DaemonOption) *Daemon {
	daemon := &Daemon{
		mutex:   &sync.Mutex{},
		session: []internalWorkerSession{},
		persist: persistent,
		childprocess: childprocess.NewChildProcessSystem(func(proc, log string) {
			fmt.Println(proc + " : " + log)
			persistent.Log(proc, "childprocess", log)
		}),
		log: log.TakeLog(func(log string) {
			persistent.Log("daemon.exe", "infor", log)
		}),
	}

	go func() {
		for {
			infor, err := system.GetInfor()
			if err != nil {
				log.PushLog("error get sysinfor : %s", err.Error())
				return
			}

			if options.Turn != nil {
				port := int32(options.Turn.Port)
				infor.TurnPort = &port
			} else {
				infor.TurnPort = nil
			}
			daemon.persist.Infor(infor)
		}
	}()
	daemon.persist.Sessions(func() []packet.WorkerSession {
		sessions := []packet.WorkerSession{}
		for _, iws := range daemon.session {
			sessions = append(sessions, packet.WorkerSession{
				Id: iws.Id,
				Timestamp: iws.Timestamp,
				Thinkmay: iws.Thinkmay,
				Sunshine: iws.Sunshine,
			})
		}
		return sessions
	})

	daemon.persist.RecvSession(func(ss *packet.WorkerSession) error {
		process := []childprocess.ProcessID{}
		var index *int 
		i := 0


		err := fmt.Errorf("no session configured")
		if ss.Thinkmay != nil {
			process,i, err = daemon.handleHub(ss.Thinkmay)
			index = &i
		}
		if ss.Sunshine != nil {
			process, err = daemon.handleSunshine(ss.Sunshine)
		}
		if err != nil {
			log.PushLog("session failed")
			return err
		}

		log.PushLog("session creation successful")
		daemon.session = append(daemon.session,
			internalWorkerSession{
				*ss, process,index,
			})

		return nil
	})

	go func() {
		for {
			ss := daemon.persist.ClosedSession()
			log.PushLog("terminating session %d",ss)
			queue := []internalWorkerSession{}
			for _, ws := range daemon.session {
				if ws.display != nil {
					media.RemoveVirtualDisplay(*ws.display)
				}
				if int(ws.Id) == ss {
					for _, pi := range ws.childprocess {
						daemon.childprocess.CloseID(pi)
					}
				} else {
					queue = append(queue, ws)
				}
			}

			if len(daemon.session) == len(queue) {
				log.PushLog("no session terminated, total session : %d",len(daemon.session))
			} else {
				daemon.session = queue
			}
		}
	}()

	return daemon
}

func (daemon *Daemon) Close() {
	daemon.childprocess.CloseAll()
	log.RemoveCallback(daemon.log)
}

func (daemon *Daemon) handleHub(current *packet.ThinkmaySession) ([]childprocess.ProcessID, int, error) {
	daemon.mutex.Lock()
	defer daemon.mutex.Unlock()

	display,index := media.StartVirtualDisplay(int(current.ScreenWidth), int(current.ScreenHeight))
	webrtcHash,displayHash :=
		string(base64.StdEncoding.EncodeToString([]byte(current.WebrtcConfig))),
		string(base64.StdEncoding.EncodeToString([]byte(display)))

	hub_path, err := path.FindProcessPath("", "hub.exe")
	if err != nil {
		return nil,0, err
	}
	cmd := []string{
		"--webrtc", webrtcHash,
		"--display", displayHash,
	}

	video, err := daemon.childprocess.NewChildProcess(exec.Command(hub_path, cmd...), true)
	if err != nil {
		return nil,0, err
	}

	return []childprocess.ProcessID{video},index, nil
}


func (daemon *Daemon) handleSunshine(current *packet.SunshineSession) ([]childprocess.ProcessID, error) {
	daemon.mutex.Lock()
	defer daemon.mutex.Unlock()

	hub_path, err := path.FindProcessPath("", "sunshine.exe")
	if err != nil {
		return nil, err
	}

	cmd := []string{
		"--username", current.Username,
		"--password", current.Password,
	}

	id, err := daemon.childprocess.NewChildProcess(exec.Command(hub_path, cmd...), true)
	if err != nil {
		return nil, err
	}

	return []childprocess.ProcessID{id}, nil
}