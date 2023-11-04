package daemon

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"

	"syscall"
	"time"

	childprocess "github.com/thinkonmay/thinkshare-daemon/child-process"
	"github.com/thinkonmay/thinkshare-daemon/persistent"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	utils "github.com/thinkonmay/thinkshare-daemon/utils/path"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

type Daemon struct {
	childprocess *childprocess.ChildProcesses
	Shutdown     chan bool
	persist      persistent.Persistent
	media        *packet.MediaDevice

	mutex   *sync.Mutex

	sessions []packet.WorkerSession
	apps     []packet.AppSession
}

func NewDaemon(persistent persistent.Persistent,
				handlePartition func(*packet.Partition)) *Daemon {
	daemon := &Daemon{
		persist:      persistent,
		Shutdown:     make(chan bool),
		childprocess: childprocess.NewChildProcessSystem(),

		mutex:   &sync.Mutex{},
		sessions: []packet.WorkerSession{},
		apps: 	  []packet.AppSession{},
	}
	go func() {
		for {
			child_log := <-daemon.childprocess.LogChan
			name := fmt.Sprintf("childprocess %d", child_log.ID)
			daemon.persist.Log(name, child_log.LogType, child_log.Log)
			fmt.Printf("%s : %s\n",name,child_log.Log)
		}
	}()
	go func() {
		for {
			out := log.TakeLog()
			daemon.persist.Log("daemon.exe", "infor", out)
			fmt.Printf("daemon.exe : %s\n",out)
		}
	}()
	go func() {
		for {
			daemon.media = media.GetDevice()
			time.Sleep(5 * time.Second)
		}
	}()
	go func() {
		infor, err := system.GetInfor()
		if err != nil {
			log.PushLog("error get sysinfor : %s", err.Error())
			return
		}

		for _,partition := range infor.Partitions {
			handlePartition(partition)
		}

		daemon.persist.Infor(infor)
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

type Manifest struct {
	ProcessID childprocess.ProcessID `json:"hub_process_id"`
}

func (manifest Manifest) Default() *Manifest {
	return &Manifest{
		ProcessID: childprocess.NullProcID,
	}
}


func (daemon *Daemon) sync(ss *packet.WorkerSessions) (ret *packet.WorkerSessions) {
	daemon.mutex.Lock()
	defer daemon.mutex.Unlock()

	ret = &packet.WorkerSessions{
		Sessions: []*packet.WorkerSession{},
		Apps:     []*packet.AppSession{},
	}

	func(){
		reset := func() {
			current := &daemon.sessions[0]
			session := &Manifest{}
			if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
				session = Manifest{}.Default()
			}
			defer func() {
				bytes, _ := json.Marshal(session)
				current.Manifest = string(bytes)
			}()

			if session.ProcessID.Valid() {
				daemon.childprocess.CloseID(childprocess.ProcessID(session.ProcessID))
			}
		}
		kill := func() {
			current := &daemon.sessions[0]
			session := &Manifest{}
			if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
				session = Manifest{}.Default()
			}
			defer func() {
				bytes, _ := json.Marshal(session)
				current.Manifest = string(bytes)
			}()

			if session.ProcessID.Valid() {
				daemon.childprocess.CloseID(childprocess.ProcessID(session.ProcessID))
			}
		}

		// TODO multiple sessions
		if len(ss.Sessions) > 1 {
			log.PushLog("number of session is more than 1, not valid")
			ret.Sessions = []*packet.WorkerSession{ss.Sessions[0]}
			return 
		} else if len(ss.Sessions) == 0 && len(daemon.sessions) > 0 {
			kill()
			daemon.sessions = []packet.WorkerSession{}
			return 
		} else if len(ss.Sessions) == 0 && len(daemon.sessions) == 0 {
			return 
		}

		desired_session := ss.Sessions[0]
		if len(ss.Sessions) == 1 && len(daemon.sessions) == 0 {
			defaultManifest, _ := json.Marshal(Manifest{}.Default())
			daemon.sessions = []packet.WorkerSession{{
				Manifest: string(defaultManifest),
			}}
		}

		current_session := &daemon.sessions[0]

		// check if sync-required feature need to resync
		if desired_session.Id != current_session.Id && desired_session.AuthConfig != "{}" {

			// reset daemon current session state if sync is required
			current_session.WebrtcConfig = desired_session.WebrtcConfig
			current_session.SignalingConfig = desired_session.SignalingConfig
			current_session.AuthConfig = desired_session.AuthConfig
			current_session.Id = desired_session.Id

			reset()
		}

		// reset daemon current session state if sync is required
		// desired.SessionLog = current.SessionLog
		desired_session.Manifest = current_session.Manifest

		ret.Sessions = []*packet.WorkerSession{ desired_session }
	}()


	return ret
}


func (daemon *Daemon) handleHub() {
	presync :=  func() (authHash string, 
						signalingHash string, 
						webrtcHash string, 
						audioHash string, 
						micHash string, 
						err error) {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		if len(daemon.sessions) == 0 {
			err = fmt.Errorf("no current session")
			return
		}

		current := &daemon.sessions[0]
		session := &Manifest{}
		if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
			session = Manifest{}.Default()
		}
		defer func() {
			bytes, _ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()


		bypass := false
		if daemon.media == nil {
			// err = fmt.Errorf("media device not ready")
			// return 
			bypass = true
		} else if daemon.media.Soundcard == nil {
			// err = fmt.Errorf("media device not ready")
			// return 
			bypass = true
		} else if daemon.media.Soundcard.Pipeline == nil {
			// err = fmt.Errorf("media device not ready")
			// return 
			bypass = true
		} else if daemon.media.Microphone == nil {
			// err = fmt.Errorf("media device not ready")
			// return 
			bypass = true
		} else if daemon.media.Microphone.Pipeline == nil {
			// err = fmt.Errorf("media device not ready")
			// return 
			bypass = true
		}

		if bypass {
			audioHash = ""
			micHash   = ""
		} else {
			audioHash = daemon.media.Soundcard.Pipeline.PipelineHash
			micHash   = daemon.media.Microphone.Pipeline.PipelineHash
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

		if len(daemon.sessions) == 0 {
			return fmt.Errorf("no current session")
		}

		current := &daemon.sessions[0]
		session := &Manifest{}
		if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
			session = Manifest{}.Default()
		}
		defer func() {
			bytes, _ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()

		session.ProcessID = id
		return nil
	}


	appsession := func() (path string, args []string, envs []string,err error) {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		if len(daemon.apps) == 0 {
			err = fmt.Errorf("no current session")
			return 
		}

		current := &daemon.apps[0]
		path, err = utils.FindProcessPath(current.Folder, current.Exe)
		if err != nil {
			return
		}

		return path,current.Args,current.Envs,nil
	}

	for {
		time.Sleep(time.Millisecond * 500)
		authHash, signaling, webrtc, audioHash,micHash, err := presync()
		if err != nil {
			continue
		} else if app_path,args,envs,err := appsession(); err == nil {
			process := exec.Command(app_path,args...)
			process.Env = envs
			daemon.childprocess.NewChildProcess(process,false)
		}


		hub_path, err := utils.FindProcessPath("", "hub.exe")
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
		if audioHash != "" {
			cmd = append(cmd, "--audio", audioHash)
		}

		id, err := daemon.childprocess.NewChildProcess(exec.Command(hub_path,cmd...),true)
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