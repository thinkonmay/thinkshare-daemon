package daemon

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

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
	Shutdown     chan bool
	persist      persistent.Persistent

	mutex *sync.Mutex

	session []internalWorkerSession
}

func NewDaemon(persistent persistent.Persistent) *Daemon {
	daemon := &Daemon{
		persist:      persistent,
		Shutdown:     make(chan bool),
		childprocess: childprocess.NewChildProcessSystem(),

		mutex:   &sync.Mutex{},
		session: []internalWorkerSession{},
	}
	go func() {
		for {
			child_log := <-daemon.childprocess.LogChan
			name := fmt.Sprintf("childprocess %d", child_log.ID)
			daemon.persist.Log(name, child_log.LogType, child_log.Log)
			fmt.Printf("%s : %s\n", name, child_log.Log)
		}
	}()
	go func() {
		for {
			out := log.TakeLog()
			daemon.persist.Log("daemon.exe", "infor", out)
			fmt.Printf("daemon.exe : %s\n", out)
		}
	}()
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
			process,err := daemon.handleHub(ss)
			if err != nil {
				daemon.persist.FailedSession(ss)
				continue
			}

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
			for _,ws := range daemon.session {
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

func (daemon *Daemon) TerminateAtTheEnd() {
	go func() {
		chann := make(chan os.Signal, 10)
		signal.Notify(chann, syscall.SIGTERM, os.Interrupt)
		<-chann

		daemon.childprocess.CloseAll()
		daemon.Shutdown <- true
	}()
}





func (daemon *Daemon) handleHub(current *packet.WorkerSession) (childprocess.ProcessID,error) {
	daemon.mutex.Lock()
	defer daemon.mutex.Unlock()

	authHash, signalingHash, webrtcHash :=
		string(base64.StdEncoding.EncodeToString([]byte(current.AuthConfig))),
		string(base64.StdEncoding.EncodeToString([]byte(current.SignalingConfig))),
		string(base64.StdEncoding.EncodeToString([]byte(current.WebrtcConfig)))

	hub_path, err := path.FindProcessPath("", "hub.exe")
	if err != nil {
		return childprocess.NullProcID,err
	}

	media.StartVirtualDisplay(int(current.ScreenWidth),int(current.ScreenHeight))
	cmd := []string{
		"--auth", authHash,
		"--grpc", signalingHash,
		"--webrtc", webrtcHash,
	}


	id, err := daemon.childprocess.NewChildProcess(exec.Command(hub_path, cmd...), true)
	if err != nil {
		return childprocess.NullProcID,err
	}

	return id,nil
}
