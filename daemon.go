package daemon

import (
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
	ground       packet.WorkerSession
}

func NewDaemon(persistent persistent.Persistent,
			   ) *Daemon {
	daemon := &Daemon{
		persist: persistent,
		Shutdown:               make(chan bool),
		childprocess:           childprocess.NewChildProcessSystem(),

		mutex: &sync.Mutex{},
		ground: packet.WorkerSession{},
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
			daemon.persist.SyncSession(daemon.sync(ss))
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

	HidProcessID int `json:"hid_process_id"`	
	HubProcessID int `json:"hub_process_id"`	
}



func (daemon *Daemon) kill(){
	session := SessionManifest{}
	if err := json.Unmarshal([]byte(daemon.ground.Manifest),&session); err != nil {
		session = SessionManifest{
			HidProcessID: 0,
			HubProcessID: 0,
		}
	}

	if session.HidProcessID != 0 {
		daemon.childprocess.CloseID(childprocess.ProcessID(session.HidProcessID))
	}
	if session.HubProcessID != 0 {
		daemon.childprocess.CloseID(childprocess.ProcessID(session.HubProcessID))
	}


	bytes,_ := json.Marshal(session)
	daemon.ground.Manifest = string(bytes)
}
func (daemon *Daemon) reset(){
	session := SessionManifest{}
	if err := json.Unmarshal([]byte(daemon.ground.Manifest),&session); err != nil {
		session = SessionManifest{
			HidProcessID: 0,
			HubProcessID: 0,
		}
	}

	if session.HubProcessID != 0 {
		daemon.childprocess.CloseID(childprocess.ProcessID(session.HubProcessID))
	}


	bytes,_ := json.Marshal(session)
	daemon.ground.Manifest = string(bytes)
}

func (daemon *Daemon) sync(ss packet.WorkerSessions)packet.WorkerSessions {
	daemon.mutex.Lock()
	defer daemon.mutex.Unlock()

	if len(ss.Sessions) > 1 {
		log.PushLog("number of session is more than 1, not valid");
		return packet.WorkerSessions{
			Sessions: []*packet.WorkerSession{&daemon.ground},
		}
	} else if len(ss.Sessions) == 0 {
		daemon.kill()	
		return ss
	}

	desired := ss.Sessions[0]
	diff := false

	diff = desired.Monitor.MonitorHandle != daemon.ground.Monitor.MonitorHandle
	diff = desired.Soundcard.DeviceID != daemon.ground.Soundcard.DeviceID
	diff = desired.WebRTCConfig != daemon.ground.WebRTCConfig
	diff = desired.SignalingConfig != daemon.ground.SignalingConfig
	diff = desired.Token != daemon.ground.Token

	if diff {
		daemon.ground = *desired		
		daemon.reset()
	}

	return packet.WorkerSessions{
		Sessions: []*packet.WorkerSession{desired},
	}
}


func (daemon *Daemon) handleHID() (){
	for {
		path, err := utils.FindProcessPath("hid", "hid.exe")
		if err != nil {
			log.PushLog("unable to find hid.exe: %s",err.Error())
			continue
		}

		free_port,err := port.GetFreePort()
		if err != nil {
			log.PushLog("unable to find open port: %s",err.Error())
			continue
		}

		daemon.mutex.Lock()
		session := SessionManifest{}
		if err := json.Unmarshal([]byte(daemon.ground.Manifest),&session); err != nil {
			session = SessionManifest{}
		}
	
		session.HidPort = free_port

		bytes,_ := json.Marshal(session)
		daemon.ground.Manifest = string(bytes)
		daemon.mutex.Unlock()

		process := exec.Command(path, fmt.Sprintf("--urls=http://localhost:%d", free_port))
		id,err := daemon.childprocess.NewChildProcess(process)
		if err != nil || id == -1{
			log.PushLog("fail to start hid process: %s",err.Error())
		}

		daemon.mutex.Lock()
		session = SessionManifest{}
		if err := json.Unmarshal([]byte(daemon.ground.Manifest),&session); err != nil {
			session = SessionManifest{}
		}
	
		session.FailCount++
		session.HidProcessID = int(id)

		bytes,_ = json.Marshal(session)
		daemon.ground.Manifest = string(bytes)
		daemon.mutex.Unlock()

		daemon.childprocess.WaitID(id)
	}
}



func (daemon *Daemon) handleHub() (){
	for {
		path, err := utils.FindProcessPath("hub/bin", "hub.exe")
		if err != nil {
			log.PushLog("unable to find hid.exe: %s",err.Error())
			continue
		}

		daemon.mutex.Lock()
		session := SessionManifest{}
		if err := json.Unmarshal([]byte(daemon.ground.Manifest),&session); err != nil {
			session = SessionManifest{}
		}


		token 	    := daemon.ground.Token
		signaling 	:= daemon.ground.SignalingConfig
		webrtc 	    := daemon.ground.WebRTCConfig
		hidport     := session.HidPort

		video := pipeline.VideoPipeline{}
		err = video.SyncPipeline(daemon.ground.Monitor)
		if err != nil {
			daemon.ground.SessionLog = append(daemon.ground.SessionLog, err.Error())
			session.FailCount++
		}

		audio := pipeline.AudioPipeline{}
		err = audio.SyncPipeline(daemon.ground.Soundcard)
		if err != nil {
			daemon.ground.SessionLog = append(daemon.ground.SessionLog, err.Error())
			session.FailCount++
		}

		daemon.ground.Pipelines = []string{audio.PipelineString,video.PipelineString}

		bytes,_ := json.Marshal(session)
		daemon.ground.Manifest = string(bytes)
		daemon.mutex.Unlock()

		if err != nil {
			continue
		}

		process := exec.Command(path,
			"--hid", 		fmt.Sprintf("localhost:%d", hidport),
			"--token", 		token,
			"--audio", 		audio.PipelineHash,
			"--video", 		video.PipelineHash,
			"--grpc", 		signaling,
			"--webrtc", 	webrtc)

		id,err := daemon.childprocess.NewChildProcess(process)
		if err != nil || id == -1{
			log.PushLog("fail to start hid process: %s",err.Error())
		}

		daemon.mutex.Lock()
		session = SessionManifest{}
		if err := json.Unmarshal([]byte(daemon.ground.Manifest),&session); err != nil {
			session = SessionManifest{}
		}
	
		session.FailCount++
		session.HubProcessID = int(id) 

		bytes,_ = json.Marshal(session)
		daemon.ground.Manifest = string(bytes)
		daemon.mutex.Unlock()
		

		daemon.childprocess.WaitID(id)
	}
}


