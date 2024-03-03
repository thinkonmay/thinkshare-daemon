package daemon

import (
	"encoding/base64"
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

	childprocess childprocess.ProcessID
}

type Daemon struct {
	childprocess *childprocess.ChildProcesses
	persist      persistent.Persistent

	mutex *sync.Mutex

	session []internalWorkerSession
	log int
}

func NewDaemon(persistent persistent.Persistent) *Daemon {
	daemon := &Daemon{
		mutex:   &sync.Mutex{},
		session: []internalWorkerSession{},
		persist:      persistent,
		childprocess: childprocess.NewChildProcessSystem(func(proc,log string) {
			persistent.Log(proc, "childprocess", log)
		}),
		log: log.TakeLog(func(log string) {
			persistent.Log("daemon.exe", "infor", log)
		}),
	}

	go func() {
		infor, err := system.GetInfor()
		if err != nil {
			log.PushLog("error get sysinfor : %s", err.Error())
			return
		}

		daemon.persist.Infor(infor)
	}()

	go func() {
		for {
			ss := daemon.persist.RecvSession()
			log.PushLog("new session")
			process, err := daemon.handleHub(ss)
			if err != nil {
				log.PushLog("session %d failed", ss.Id)
				daemon.persist.FailedSession(ss)
				continue
			}

			log.PushLog("session creation successful")
			daemon.session = append(daemon.session,
				internalWorkerSession{
					*ss, process,
				})
		}
	}()
	go func() {
		for {
			ss := daemon.persist.ClosedSession()
			queue := []internalWorkerSession{}
			for _, ws := range daemon.session {
				if int(ws.Id) == ss {
					daemon.childprocess.CloseID(ws.childprocess)
				} else {
					queue = append(queue, ws)
				}
			}
			daemon.session = queue
		}
	}()

	return daemon
}

func (daemon *Daemon) Close() {
	daemon.childprocess.CloseAll()
	log.RemoveCallback(daemon.log)
}

func (daemon *Daemon) handleHub(current *packet.WorkerSession) (childprocess.ProcessID, error) {
	daemon.mutex.Lock()
	defer daemon.mutex.Unlock()

	authHash, signalingHash, webrtcHash :=
		string(base64.StdEncoding.EncodeToString([]byte(current.AuthConfig))),
		string(base64.StdEncoding.EncodeToString([]byte(current.SignalingConfig))),
		string(base64.StdEncoding.EncodeToString([]byte(current.WebrtcConfig)))

	hub_path, err := path.FindProcessPath("", "hub.exe")
	if err != nil {
		return childprocess.NullProcID, err
	}

	media.StartVirtualDisplay(int(current.ScreenWidth), int(current.ScreenHeight))
	cmd := []string{
		"--auth", authHash,
		"--grpc", signalingHash,
		"--webrtc", webrtcHash,
	}

	id, err := daemon.childprocess.NewChildProcess(exec.Command(hub_path, cmd...), true)
	if err != nil {
		return childprocess.NullProcID, err
	}

	return id, nil
}
