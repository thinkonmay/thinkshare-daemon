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
	"github.com/thinkonmay/thinkshare-daemon/utils/path"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

type Daemon struct {
	childprocess *childprocess.ChildProcesses
	Shutdown     chan bool
	persist      persistent.Persistent

	mutex *sync.Mutex

	session []packet.WorkerSession
}

func NewDaemon(persistent persistent.Persistent) *Daemon {
	daemon := &Daemon{
		persist:      persistent,
		Shutdown:     make(chan bool),
		childprocess: childprocess.NewChildProcessSystem(),

		mutex:   &sync.Mutex{},
		session: []packet.WorkerSession{},
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
			daemon.handleHub(ss)
			daemon.session = append(daemon.session, ss)
			daemon.persist.SyncSession(daemon.session)
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





func (daemon *Daemon) handleHub(n packet.WorkerSession) error {
	presync := func(current packet.WorkerSession) (authHash string,
													signalingHash string,
													webrtcHash string,
													err error) {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		authHash, signalingHash, webrtcHash =
			string(base64.StdEncoding.EncodeToString([]byte(current.AuthConfig))),
			string(base64.StdEncoding.EncodeToString([]byte(current.SignalingConfig))),
			string(base64.StdEncoding.EncodeToString([]byte(current.WebrtcConfig)))

		return
	}

	aftersync := func(id childprocess.ProcessID) error {
		return nil
	}


	authHash, signaling, webrtc, err := presync(n)
	if err != nil {
		return err
	}

	hub_path, err := path.FindProcessPath("", "hub.exe")
	if err != nil {
		return err
	}

	cmd := []string{
		"--auth", authHash,
		"--grpc", signaling,
		"--webrtc", webrtc,
	}

	id, err := daemon.childprocess.NewChildProcess(exec.Command(hub_path, cmd...), true)
	if err != nil {
		return err
	}

	return aftersync(id)
}
