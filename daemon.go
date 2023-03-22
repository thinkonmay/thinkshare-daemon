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

	"github.com/thinkonmay/conductor/protocol/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/child-process"
	"github.com/thinkonmay/thinkshare-daemon/persistent"
	"github.com/thinkonmay/thinkshare-daemon/pipeline"
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

	mutex        *sync.Mutex
	current       []packet.WorkerSession
}

func NewDaemon(persistent persistent.Persistent) *Daemon {
	daemon := &Daemon{
		persist: persistent,
		Shutdown:               make(chan bool),
		childprocess:           childprocess.NewChildProcessSystem(),

		mutex: &sync.Mutex{},
		current: []packet.WorkerSession{},
	}
	go func ()  {
		for {
			child_log := <-daemon.childprocess.LogChan
			daemon.persist.Log(fmt.Sprintf("childprocess %d",child_log.ID),child_log.LogType,child_log.Log)
		}
	}()
	go func ()  {
		for {
			out := log.TakeLog()
			daemon.persist.Log("daemon.exe","infor",out)
		}
	}()
	go func ()  {
		for {
			media := media.GetDevice()
			daemon.persist.Media(media)
			time.Sleep(10 * time.Second)
		}
	}()
	go func ()  {
		for {
			infor,err := system.GetInfor()
			if err != nil {
				log.PushLog("error get sysinfor : %s",err.Error())
				continue
			}
			daemon.persist.Infor(infor)
			time.Sleep(10 * time.Second)
		}
	}()


	go func ()  {
		for {
			ss := daemon.persist.RecvSession()
			result := daemon.sync(*ss)
			daemon.persist.SyncSession(&result)
			time.Sleep(1 * time.Second)
		}
	}()


	go daemon.handleHID()
	go daemon.handleHub()
	return daemon
}





func (daemon *Daemon)TerminateAtTheEnd() {
	go func ()  {
		chann := make(chan os.Signal, 10)
		signal.Notify(chann, syscall.SIGTERM, os.Interrupt)
		<-chann

		daemon.childprocess.CloseAll()
		time.Sleep(100 * time.Millisecond)
		daemon.Shutdown <- true
	}()
}







type SessionManifest struct {
	HidPort int `json:"hid_port"`	
	FailCount int `json:"fail_count"`	

	HidProcessID childprocess.ProcessID `json:"hid_process_id"`	
	HubProcessID childprocess.ProcessID `json:"hub_process_id"`	
}

func (manifest SessionManifest)Default() *SessionManifest{
	return &SessionManifest{
		HidPort: 0,
		FailCount: 0,
		HidProcessID: childprocess.NullProcID,
		HubProcessID: childprocess.NullProcID,
	}
}







func (daemon *Daemon) sync(ss packet.WorkerSessions)packet.WorkerSessions {
	daemon.mutex.Lock()
	defer daemon.mutex.Unlock()

	reset := func(){
		current := &daemon.current[0]
		session := &SessionManifest{}
		if err := json.Unmarshal([]byte(current.Manifest),session); err != nil {
			session = SessionManifest{}.Default()
		}
		defer func ()  {
			bytes,_ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()

		if session.HubProcessID.Valid() {
			daemon.childprocess.CloseID(childprocess.ProcessID(session.HubProcessID))
		}
	}
	kill := func(){
		current := &daemon.current[0]
		session := &SessionManifest{}
		if err := json.Unmarshal([]byte(current.Manifest),session); err != nil {
			session = SessionManifest{}.Default()
		}
		defer func ()  {
			bytes,_ := json.Marshal(session)
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
		log.PushLog("number of session is more than 1, not valid");
		return packet.WorkerSessions{ Sessions: []*packet.WorkerSession{ss.Sessions[0]}, }
	} else if len(ss.Sessions) == 0 && len(daemon.current) > 0 {
		kill()	
		daemon.current = []packet.WorkerSession{}
		return ss
	} else if len(ss.Sessions) == 0 && len(daemon.current) == 0 {
		return ss
	} 

	desired := ss.Sessions[0]
	if len(ss.Sessions) == 1 && len(daemon.current) == 0 {
		defaultManifest,_ := json.Marshal(SessionManifest{}.Default())
		daemon.current = []packet.WorkerSession{{
			WebRTCConfig: desired.WebRTCConfig,	
			SignalingConfig: desired.SignalingConfig ,
			Token: desired.Token,
			Monitor: desired.Monitor,
			Soundcard: desired.Soundcard,
			Pipelines: make([]string, 0),
			SessionLog: make([]string, 0),
			Manifest: string(defaultManifest),
		}}
	}



	current := &daemon.current[0]

	// check if sync-required feature need to resync
	if  desired.Monitor.MonitorHandle 	!= current.Monitor.MonitorHandle ||
		desired.Soundcard.DeviceID 		!= current.Soundcard.DeviceID ||
		desired.WebRTCConfig 			!= current.WebRTCConfig ||
		desired.SignalingConfig 		!= current.SignalingConfig || 
		desired.Token 					!= current.Token {

		// reset daemon current session state if sync is required
		current.Monitor.MonitorHandle 	= desired.Monitor.MonitorHandle 
		current.Soundcard.DeviceID 		= desired.Soundcard.DeviceID 
		current.WebRTCConfig 			= desired.WebRTCConfig 
		current.SignalingConfig 		= desired.SignalingConfig 
		current.Token 					= desired.Token

		reset()
	}

	// reset daemon current session state if sync is required
	desired.SessionLog = current.SessionLog
	desired.Manifest   = current.Manifest
	desired.Pipelines  = current.Pipelines

	return packet.WorkerSessions{
		Sessions: []*packet.WorkerSession{desired},
	}
}


func (daemon *Daemon) handleHID() (){
	presync := func() (string,int,error)  {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		current := &daemon.current[0]
		session := &SessionManifest{}
		if err := json.Unmarshal([]byte(current.Manifest),session); err != nil {
			session = SessionManifest{}.Default()
		}
		defer func ()  {
			bytes,_ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()

	
		free_port,err := port.GetFreePort()
		if err != nil {
			current.SessionLog = append(current.SessionLog, fmt.Sprintf("unable to find open port: %s",err.Error()))
			session.FailCount++
			return "",0,err
		}

		path, err := utils.FindProcessPath("hid", "hid.exe")
		if err != nil {
			current.SessionLog = append(current.SessionLog, fmt.Sprintf("unable to find hid port: %s",err.Error()))
			session.FailCount++
			return "",0,err
		}

		if session.HidPort == 0 {
			session.HidPort = free_port
		}

		current.SessionLog = append(current.SessionLog, fmt.Sprintf("found available hid port : %d",session.HidPort))
		current.SessionLog = append(current.SessionLog, fmt.Sprintf("inialize hid.exe at path : %s",path))
		return path,session.HidPort,nil
	}
	aftersync := func(id childprocess.ProcessID) {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		current := &daemon.current[0]
		session := &SessionManifest{}
		if err := json.Unmarshal([]byte(current.Manifest),session); err != nil {
			session = SessionManifest{}.Default()
		}
		defer func ()  {
			bytes,_ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()

	
		if !session.HidProcessID.Valid() {
			current.SessionLog = append(current.SessionLog, "fail to start hid.exe")
			session.FailCount++
		}

		session.HidProcessID = id
		current.SessionLog = append(current.SessionLog, fmt.Sprintf("started hid.exe with processID %d",id))
	}

	for {
		if len(daemon.current) == 0 {
			time.Sleep(time.Millisecond * 100)
			continue
		} 



		path,free_port,err := presync()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}
		process := exec.Command(path, fmt.Sprintf("--urls=http://localhost:%d", free_port))
		id,err := daemon.childprocess.NewChildProcess(process)
		aftersync(id)

		if err != nil {
			log.PushLog("fail to start hid process: %s",err.Error())
		} else {
			daemon.childprocess.WaitID(id)
		}
	}
}



func (daemon *Daemon) handleHub() (){
	presync := func() (path string,token string, signalingHash string, webrtcHash string, audioHash string, videoHash string, hidport int,err error){
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		current := &daemon.current[0]
		session := &SessionManifest{}
		if err := json.Unmarshal([]byte(current.Manifest),session); err != nil {
			session = SessionManifest{}.Default()
		}
		defer func ()  {
			bytes,_ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()

		path, err = utils.FindProcessPath("hub/bin", "hub.exe")
		if err != nil {
			current.SessionLog = append(current.SessionLog, fmt.Sprintf("unable to find hid.exe: %s",err.Error()))
			session.FailCount++
			return "","","","","","",0,err
		}


		if session.HidPort == 0 {
			current.SessionLog = append(current.SessionLog, fmt.Sprintf("invalid hid port: %d",session.HidPort))
			session.FailCount++
		}

		token 	    = current.Token
		hidport     = session.HidPort

		signalingBytes,_ := base64.StdEncoding.DecodeString(current.SignalingConfig)
		webrtcBytes,_ 	 := base64.StdEncoding.DecodeString(current.WebRTCConfig)
		signalingHash,webrtcHash  = string(signalingBytes),string(webrtcBytes)

		video := pipeline.VideoPipeline{}
		err = video.SyncPipeline(current.Monitor)
		if err != nil {
			current.SessionLog = append(current.SessionLog, fmt.Sprintf("failed to test video pipeline: %s",err.Error()))
			session.FailCount++
			return "","","","","","",0,err
		}

		audio := pipeline.AudioPipeline{}
		err = audio.SyncPipeline(current.Soundcard)
		if err != nil {
			current.SessionLog = append(current.SessionLog, fmt.Sprintf("failed to test audio pipeline: %s",err.Error()))
			session.FailCount++
			return "","","","","","",0,err
		}

		current.SessionLog = append(current.SessionLog, "tested video and audio pipeline")
		current.SessionLog = append(current.SessionLog, fmt.Sprintf("inialize hub.exe at path : %s",path))
		current.Pipelines = []string{audio.PipelineString,video.PipelineString}
		audioHash = audio.PipelineHash
		videoHash = video.PipelineHash

		return
	}

	aftersync := func (id childprocess.ProcessID)  {
		daemon.mutex.Lock()
		defer daemon.mutex.Unlock()

		current := &daemon.current[0]
		session := &SessionManifest{}
		if err := json.Unmarshal([]byte(current.Manifest),session); err != nil {
			session = SessionManifest{}.Default()
		}
		defer func ()  {
			bytes,_ := json.Marshal(session)
			current.Manifest = string(bytes)
		}()
	
		if !session.HubProcessID.Valid() {
			current.SessionLog = append(current.SessionLog, "fail to start hub.exe")
			session.FailCount++
		}

		current.SessionLog = append(current.SessionLog, fmt.Sprintf("started hub.exe with processID %d",id))
		session.HubProcessID = id
	}
	for {
		if len(daemon.current) == 0 {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		path,token,signaling,webrtc,audioHash,videoHash,hidport,err :=  presync()
		if err != nil {
			log.PushLog("invalid initialization")	
			continue
		}

		process := exec.Command(path,
			"--hid", 		fmt.Sprintf("localhost:%d", hidport),
			"--token", 		token,
			"--audio", 		audioHash,
			"--video", 		videoHash,
			"--grpc", 		signaling,
			"--webrtc", 	webrtc)

		id,err := daemon.childprocess.NewChildProcess(process)
		aftersync(id)

		if err != nil{
			log.PushLog("fail to start hid process: %s",err.Error())
		} else {
			daemon.childprocess.WaitID(id)
		}
	}
}


