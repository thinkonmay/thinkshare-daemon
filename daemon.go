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
	"github.com/thinkonmay/thinkshare-daemon/utils/pipeline"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	utils "github.com/thinkonmay/thinkshare-daemon/utils/path"
	"github.com/thinkonmay/thinkshare-daemon/utils/port"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

type Daemon struct {
	childprocess *childprocess.ChildProcesses
	Shutdown     chan bool
	persist      persistent.Persistent

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
			devices := media.GetDevice()
			for _, soundcard := range devices.Soundcards {
				audio, err := pipeline.AudioPipeline(soundcard)
				if err != nil {
					continue
				}

				soundcard.Pipeline = audio
			}
			for _, monitor := range devices.Monitors {
				video, err := pipeline.VideoPipeline(monitor)
				if err != nil {
					continue
				}

				monitor.Pipeline = video
			}
			for _, mic := range devices.Microphones {
				video, err := pipeline.MicPipeline(mic)
				if err != nil {
					continue
				}

				mic.Pipeline = video
			}

			reset := true
			for _, m := range devices.Monitors {
				if m.Pipeline != nil {
					reset = false
				}
			}

			if reset {
				media.ResetVirtualDisplay()
				continue
			}

			daemon.persist.Media(devices)
			time.Sleep(10 * time.Minute)
		}
	}()
	go func() {
		for {
			infor, err := system.GetInfor()
			if err != nil {
				log.PushLog("error get sysinfor : %s", err.Error())
				continue
			}


			for _,partition := range infor.Partitions {
				handlePartition(partition)
			}

			daemon.persist.Infor(infor)
			time.Sleep(10 * time.Minute)
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

	go daemon.handleHID()
	go daemon.handleHub()
	go daemon.handleApp()
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

type SessionManifest struct {
	HidPort   int `json:"hid_port"`
	FailCount int `json:"fail_count"`

	HidProcessID childprocess.ProcessID `json:"hid_process_id"`
	HubProcessID childprocess.ProcessID `json:"hub_process_id"`
}

func (manifest SessionManifest) Default() *SessionManifest {
	return &SessionManifest{
		HidPort:      0,
		FailCount:    0,
		HidProcessID: childprocess.NullProcID,
		HubProcessID: childprocess.NullProcID,
	}
}

type AppManifest struct {
	FailCount int `json:"fail_count"`
	ProcessID childprocess.ProcessID `json:"process_id"`
}

func (manifest AppManifest) Default() *AppManifest {
	return &AppManifest{
		ProcessID: childprocess.NullProcID,
	}
}
func (daemon *Daemon) sync(ss *packet.WorkerSessions) (ret *packet.WorkerSessions) {
	daemon.mutex.Lock()
	defer daemon.mutex.Unlock()

	ret = &packet.WorkerSessions{
		Sessions: []*packet.WorkerSession{},
		Apps: []*packet.AppSession{},
	}

	func(){
		reset := func() {
			current := &daemon.sessions[0]
			session := &SessionManifest{}
			if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
				session = SessionManifest{}.Default()
			}
			defer func() {
				bytes, _ := json.Marshal(session)
				current.Manifest = string(bytes)
			}()

			if session.HubProcessID.Valid() {
				daemon.childprocess.CloseID(childprocess.ProcessID(session.HubProcessID))
			}
		}
		kill := func() {
			current := &daemon.sessions[0]
			session := &SessionManifest{}
			if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
				session = SessionManifest{}.Default()
			}
			defer func() {
				bytes, _ := json.Marshal(session)
				current.Manifest = string(bytes)
			}()

			if session.HidProcessID.Valid() {
				daemon.childprocess.CloseID(childprocess.ProcessID(session.HidProcessID))
			}
			if session.HubProcessID.Valid() {
				daemon.childprocess.CloseID(childprocess.ProcessID(session.HubProcessID))
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
			defaultManifest, _ := json.Marshal(SessionManifest{}.Default())
			daemon.sessions = []packet.WorkerSession{{
				Manifest: string(defaultManifest),
			}}
		}

		current_session := &daemon.sessions[0]

		// check if sync-required feature need to resync
		if desired_session.Id != current_session.Id && desired_session.AuthConfig != "{}" {

			// reset daemon current session state if sync is required
			current_session.MediaConfig = desired_session.MediaConfig
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






	func ()  {
		reset := func() {
			current := &daemon.apps[0]
			app := &AppManifest{}
			if err := json.Unmarshal([]byte(current.Manifest), app); err != nil {
				app = AppManifest{}.Default()
			}
			defer func() {
				bytes, _ := json.Marshal(app)
				current.Manifest = string(bytes)
			}()

			if app.ProcessID.Valid() {
				daemon.childprocess.CloseID(childprocess.ProcessID(app.ProcessID))
			}
		}
		kill := func() {
			current := &daemon.apps[0]
			session := &AppManifest{}
			if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
				session = AppManifest{}.Default()
			}
			defer func() {
				bytes, _ := json.Marshal(session)
				current.Manifest = string(bytes)
			}()

			if session.ProcessID.Valid() {
				daemon.childprocess.CloseID(childprocess.ProcessID(session.ProcessID))
			}
		}

		if len(ss.Apps) > 1 {
			log.PushLog("number of appp is more than 1, not valid")
			ret.Apps = []*packet.AppSession{ss.Apps[0]}
			return 
		} else if len(ss.Apps) == 0 && len(daemon.apps) > 0 {
			kill()
			daemon.apps = []packet.AppSession{}
			return
		} else if len(ss.Apps) == 0 && len(daemon.apps) == 0 {
			return
		}

		desired_app := ss.Apps[0]
		if len(ss.Apps) == 1 && len(daemon.apps) == 0 {
			defaultManifest, _ := json.Marshal(AppManifest{}.Default())
			daemon.apps = []packet.AppSession{{
				Manifest: string(defaultManifest),
			}}
		}

		current_app := &daemon.apps[0]
		if desired_app.Id != current_app.Id {
			current_app.Envs =  desired_app.Envs
			current_app.Args = desired_app.Args
			current_app.Exe = desired_app.Exe
			current_app.Folder = desired_app.Folder
			current_app.Id = desired_app.Id

			reset()
		}
	}()




	return ret
}

func (daemon *Daemon) handleHID() {
	presync := func() (string, int, error) {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		if len(daemon.sessions) == 0 {
			return "", 0, fmt.Errorf("no current session")
		}

		current := &daemon.sessions[0]
		session := &SessionManifest{}
		if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
			session = SessionManifest{}.Default()
		}
		defer func() {
			bytes, _ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()

		free_port, err := port.GetFreePort()
		if err != nil {
			// current.SessionLog = append(current.SessionLog, fmt.Sprintf("unable to find open port: %s",err.Error()))
			session.FailCount++
			return "", 0, err
		}

		path, err := utils.FindProcessPath("", "dotnet")
		if err != nil {
			// current.SessionLog = append(current.SessionLog, fmt.Sprintf("unable to find hid port: %s",err.Error()))
			session.FailCount++
			return "", 0, err
		}

		if session.HidPort == 0 {
			session.HidPort = free_port
		}

		// current.SessionLog = append(current.SessionLog, fmt.Sprintf("found available hid port : %d",session.HidPort))
		// current.SessionLog = append(current.SessionLog, fmt.Sprintf("inialize hid.exe at path : %s",path))
		return path, session.HidPort, nil
	}
	aftersync := func(id childprocess.ProcessID) error {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		if len(daemon.sessions) == 0 {
			return fmt.Errorf("no current session")
		}

		current := &daemon.sessions[0]
		session := &SessionManifest{}
		if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
			session = SessionManifest{}.Default()
		}
		defer func() {
			bytes, _ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()

		if !session.HidProcessID.Valid() {
			// current.SessionLog = append(current.SessionLog, "fail to start hid.exe")
			session.FailCount++
		}

		session.HidProcessID = id
		// current.SessionLog = append(current.SessionLog, fmt.Sprintf("started hid.exe with processID %d",id))
		return nil
	}

	for {
		time.Sleep(time.Millisecond * 100)

		path, free_port, err := presync()
		if err != nil || path == "" {
			continue
		}
		process := exec.Command(path, 
			"run", "--project",
			"..\\hid\\",
			fmt.Sprintf("--urls=http://localhost:%d", free_port))
		id, err := daemon.childprocess.NewChildProcess(process,true)
		if err != nil {
			log.PushLog("fail to start hid process: %s", err.Error())
			continue
		}
		err = aftersync(id)

		if err != nil {
			log.PushLog("fail to start hid process: %s", err.Error())
		} else {
			daemon.childprocess.WaitID(id)
		}
	}
}

func (daemon *Daemon) handleHub() {
	presync :=  func() (path string, 
						authHash string, 
						signalingHash string, 
						webrtcHash string, 
						audioHash string, 
						videoHash string, 
						micHash string, 
						hidport int, 
						err error) {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		if len(daemon.sessions) == 0 {
			err = fmt.Errorf("no current session")
			return
		}

		current := &daemon.sessions[0]
		session := &SessionManifest{}
		if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
			session = SessionManifest{}.Default()
		}
		defer func() {
			bytes, _ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()

		path, err = utils.FindProcessPath("", "hub.exe")
		if err != nil {
			session.FailCount++
			return
		}

		if session.HidPort == 0 {
			session.FailCount++
			err = fmt.Errorf("hid port not available")
			return
		}

		hidport = session.HidPort

		authBytes := base64.StdEncoding.EncodeToString([]byte(current.AuthConfig))
		signalingBytes := base64.StdEncoding.EncodeToString([]byte(current.SignalingConfig))
		webrtcBytes := base64.StdEncoding.EncodeToString([]byte(current.WebrtcConfig))
		signalingHash, webrtcHash, authHash = string(signalingBytes), string(webrtcBytes), string(authBytes)

		// current.SessionLog = append(current.SessionLog, "tested video and audio pipeline")
		// current.SessionLog = append(current.SessionLog, fmt.Sprintf("inialize hub.exe at path : %s",path))
		if current.MediaConfig == nil {
			err = fmt.Errorf("invalid pipeline")
			return
		} else if current.MediaConfig.Soundcard == nil ||
			current.MediaConfig.Monitor == nil {
			err = fmt.Errorf("invalid pipeline")
			return
		} else if current.MediaConfig.Soundcard.Pipeline == nil ||
			current.MediaConfig.Monitor.Pipeline == nil {
			err = fmt.Errorf("invalid pipeline")
			return
		} else if current.MediaConfig.Soundcard.Pipeline.PipelineHash == "" ||
			current.MediaConfig.Monitor.Pipeline.PipelineHash == "" {
			err = fmt.Errorf("invalid pipeline")
			return
		}

		audioHash = current.MediaConfig.Soundcard.Pipeline.PipelineHash
		videoHash = current.MediaConfig.Monitor.Pipeline.PipelineHash
		micHash   = current.MediaConfig.Microphone.Pipeline.PipelineHash

		return
	}

	aftersync := func(id childprocess.ProcessID) error {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		if len(daemon.sessions) == 0 {
			return fmt.Errorf("no current session")
		}

		current := &daemon.sessions[0]
		session := &SessionManifest{}
		if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
			session = SessionManifest{}.Default()
		}
		defer func() {
			bytes, _ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()

		if !session.HubProcessID.Valid() {
			session.FailCount++
		}

		session.HubProcessID = id
		return nil
	}
	for {
		time.Sleep(time.Millisecond * 500)
		path, authHash, signaling, webrtc, audioHash, videoHash,micHash, hidport, err := presync()
		if err != nil {
			continue
		}

		process := exec.Command(path,
			"--hid", fmt.Sprintf("localhost:%d", hidport),
			"--auth", authHash,
			"--audio", audioHash,
			"--video", videoHash,
			"--mic", micHash,
			"--grpc", signaling,
			"--webrtc", webrtc)

		id, err := daemon.childprocess.NewChildProcess(process,true)
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

func (daemon *Daemon) handleApp() {

	presync := func() (path string, args []string, envs []string,err error) {
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

	aftersync := func(id childprocess.ProcessID) error {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		if len(daemon.apps) == 0 {
			return fmt.Errorf("no current session")
		}

		current := &daemon.apps[0]
		session := &AppManifest{}
		if err := json.Unmarshal([]byte(current.Manifest), session); err != nil {
			session = (&AppManifest{}).Default()
		}
		defer func() {
			bytes, _ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()

		if !session.ProcessID.Valid() {
			session.FailCount++
		}

		session.ProcessID = id
		return nil
	}

	for {
		time.Sleep(time.Millisecond * 500)
		path,args,envs,err := presync()
		if path == "" || err != nil{
			continue
		}

		process := exec.Command(path,args...)
		process.Env = envs
		id, err := daemon.childprocess.NewChildProcess(process,false)
		if err != nil {
			log.PushLog("fail to start app process: %s", err.Error())
			continue
		}
		err = aftersync(id)

		if err != nil {
			log.PushLog("fail to start app process: %s", err.Error())
		} else {
			daemon.childprocess.WaitID(id)
		}
	}
}

