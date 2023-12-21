package daemon

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"

	"syscall"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/childprocess"
	"github.com/thinkonmay/thinkshare-daemon/persistent"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	apps "github.com/thinkonmay/thinkshare-daemon/utils/app"
	"github.com/thinkonmay/thinkshare-daemon/utils/backup"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/path"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

type Daemon struct {
	childprocess *childprocess.ChildProcesses
	Shutdown     chan bool
	persist      persistent.Persistent
	media        *packet.MediaDevice

	mutex *sync.Mutex

	session *packet.WorkerSession
	app     *packet.AppSession
}

func NewDaemon(persistent persistent.Persistent,
	handlePartition func(*packet.Partition)) *Daemon {
	daemon := &Daemon{
		persist:      persistent,
		Shutdown:     make(chan bool),
		childprocess: childprocess.NewChildProcessSystem(),

		mutex:   &sync.Mutex{},
		session: nil,
		app:     nil,
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
		for {
			daemon.media = media.GetDevice()
			time.Sleep(30 * time.Second)
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
			partitions, err := system.GetPartitions()
			if err == nil {
				for _, partition := range partitions {
					handlePartition(partition)
				}
			}

			time.Sleep(10 * time.Second)
		}
	}()

	go func() {
		for {
			ss := daemon.persist.RecvSession()
			if ss == nil {
				break
			}
			result := daemon.sync(ss)
			daemon.persist.SyncSession(result)
		}
	}()

	go daemon.handleHub()
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

func Default() *packet.Manifest {
	return &packet.Manifest{
		ProcessId: childprocess.NullProcID,
	}
}

func (daemon *Daemon) sync(ss *packet.WorkerSessions) *packet.WorkerSessions {
	daemon.mutex.Lock()
	defer daemon.mutex.Unlock()

	kill := func() {
		manifest := daemon.session.Manifest
		if !childprocess.ProcessID(manifest.ProcessId).Valid() { return }
		daemon.childprocess.CloseID(childprocess.ProcessID(manifest.ProcessId))
	}

	if ss.App != nil && daemon.app == nil {
		daemon.app = ss.App
		log.PushLog("start running backup on folder %s",ss.App.BackupFolder)
		if ss.App.BackupFolder != "none" {
			backup.StartBackup(ss.App.BackupFolder, "D:/thinkmay_backup.zip")
		}
		if ss.App.AppPath != "none" {
			apps.StartApp(ss.App.AppPath, ss.App.AppArgs...)
		}
	} else if ss.App == nil && daemon.app != nil {
		log.PushLog("stop running backup")
		daemon.app = nil
		backup.StopBackup()
	}

	if ss.Session == nil  {
		if  daemon.session != nil {
			kill()
			daemon.session = nil
		}
	} else {
		if  		daemon.session 			   == nil &&
					ss.Session.AuthConfig 	   != "" && ss.Session.AuthConfig 	   != "{}" && 
					ss.Session.SignalingConfig != "" && ss.Session.SignalingConfig != "{}" &&
					ss.Session.WebrtcConfig    != "" && ss.Session.WebrtcConfig    != "{}" {

			daemon.session = &packet.WorkerSession{ Manifest: Default(), }
			daemon.session.WebrtcConfig 		= ss.Session.WebrtcConfig
			daemon.session.SignalingConfig 		= ss.Session.SignalingConfig
			daemon.session.AuthConfig 			= ss.Session.AuthConfig
			daemon.session.Id 					= ss.Session.Id
		} else if   ss.Session.Id 			   != daemon.session.Id &&
					ss.Session.AuthConfig 	   != "" && ss.Session.AuthConfig 	   != "{}" && 
					ss.Session.SignalingConfig != "" && ss.Session.SignalingConfig != "{}" &&
					ss.Session.WebrtcConfig    != "" && ss.Session.WebrtcConfig    != "{}" {

			daemon.session.WebrtcConfig 		= ss.Session.WebrtcConfig
			daemon.session.SignalingConfig 		= ss.Session.SignalingConfig
			daemon.session.AuthConfig 			= ss.Session.AuthConfig
			daemon.session.Id 					= ss.Session.Id
			kill()
		}

		ss.Session.Manifest = daemon.session.Manifest
	}

	return ss
}

func (daemon *Daemon) handleHub() {
	presync := func() (authHash string,
		signalingHash string,
		webrtcHash string,
		audioHash string,
		micHash string,
		err error) {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		current := daemon.session
		if current == nil {
			err = fmt.Errorf("no current session")
			return
		} else if current.AuthConfig == "" || 
				  current.SignalingConfig == "" || 
				  current.WebrtcConfig == ""{
			err = fmt.Errorf("no current session")
			return
		}

		bypass := false
		if daemon.media == nil {
			bypass = true
		} else if daemon.media.Soundcard == nil {
			bypass = true
		} else if daemon.media.Soundcard.Pipeline == nil {
			bypass = true
		} else if daemon.media.Microphone == nil {
			bypass = true
		} else if daemon.media.Microphone.Pipeline == nil {
			bypass = true
		}

		if bypass {
			audioHash = ""
			micHash = ""
		} else {
			audioHash = daemon.media.Soundcard.Pipeline.PipelineHash
			micHash = daemon.media.Microphone.Pipeline.PipelineHash
		}

		authHash, signalingHash, webrtcHash =
			string(base64.StdEncoding.EncodeToString([]byte(current.AuthConfig))),
			string(base64.StdEncoding.EncodeToString([]byte(current.SignalingConfig))),
			string(base64.StdEncoding.EncodeToString([]byte(current.WebrtcConfig)))

		return
	}

	aftersync := func(id childprocess.ProcessID) error {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		if daemon.session == nil {
			return fmt.Errorf("no current session")
		}

		daemon.session.Manifest.ProcessId = int64(id)
		return nil
	}


	for {
		time.Sleep(time.Millisecond * 500)
		authHash, signaling, webrtc, audioHash, micHash, err := presync()
		if err != nil {
			continue
		}

		hub_path, err := path.FindProcessPath("", "hub.exe")
		if err != nil {
			continue
		}

		cmd := []string{
			"--auth", authHash,
			"--grpc", signaling,
			"--webrtc", webrtc,
		}

		if micHash != "" {
			cmd = append(cmd, "--mic", micHash)
		}
		if audioHash != "" {
			cmd = append(cmd, "--audio", audioHash)
		}

		id, err := daemon.childprocess.NewChildProcess(exec.Command(hub_path, cmd...), true)
		if err != nil {
			log.PushLog("fail to start hub process: %s", err.Error())
			continue
		}
		err = aftersync(id)

		if err != nil {
			log.PushLog("fail to start hub process: %s", err.Error())
		} else {
			daemon.childprocess.WaitID(id)
		}
	}
}
