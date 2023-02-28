package daemon

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"

	"syscall"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thinkonmay/thinkshare-daemon/api"
	"github.com/thinkonmay/thinkshare-daemon/api/session"
	childprocess "github.com/thinkonmay/thinkshare-daemon/child-process"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
	"github.com/thinkonmay/thinkshare-daemon/utils"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

type Daemon struct {
	LogURL                 string
	SessionSettingURL      string
	SessionRegistrationURL string

	ServerToken  string


	Childprocess *childprocess.ChildProcesses
	Shutdown     chan bool


	WebRTCHub struct {
		maxProcCount   int
		procs 		   []int

		sessionToken   string
		info  		  *session.Session
	}
	HID struct {
		port int

	}
}

func NewDaemon(domain string) *Daemon {
	dm := &Daemon{
		ServerToken:            "none",

		SessionRegistrationURL: fmt.Sprintf("https://%s/api/worker", domain),
		SessionSettingURL:      fmt.Sprintf("https://%s/api/session/setting", domain),
		LogURL:                 fmt.Sprintf("wss://%s/api/worker/log", domain),

		Shutdown:               make(chan bool),
		Childprocess:           childprocess.NewChildProcessSystem(),

		WebRTCHub: struct{maxProcCount int; procs []int; sessionToken string; info *session.Session}{
			maxProcCount: 0,
			procs: make([]int, 0),
			sessionToken: "none",
			info: nil,
		},

		HID: struct{port int}{
			port: 25678,
		},
	}



	go func ()  {
		for {
			child_log := <-dm.Childprocess.LogChan
			log.PushLog(fmt.Sprintf("childprocess (%d) (%s): %s",child_log.ID,child_log.LogType,child_log.Log))
		}
	}()


	return dm
}





func (dm *Daemon) GetServerToken(sys *system.SysInfo) (err error) {
	if dm.ServerToken, err = api.GetServerToken(dm.SessionRegistrationURL,sys); err != nil {
		log.PushLog("unable to get server token :%s\n", err.Error())
		return err
	}
	return nil;
}

func (daemon *Daemon)DefaultLogHandler(enableLogfile bool, enableWebscoketLog bool) {
	log_file,err := os.OpenFile("./log.txt", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.PushLog("error open log file to write: %s",err.Error())
		log_file = nil
		err = nil
	} else {
		log_file.Truncate(0);
	}


	var wsclient *websocket.Conn = nil
	go func ()  {
		var wserr error
		for {
			time.Sleep(5 * time.Second)
			if daemon.ServerToken == "none" || wsclient != nil {
				continue;
			}



			wsclient, _, wserr = websocket.DefaultDialer.Dial(daemon.LogURL,http.Header{
				"Authorization": []string{fmt.Sprintf("Bearer %s",daemon.ServerToken)},
			})

			if wserr != nil {
				log.PushLog("error setup log websocket : %s",wserr.Error())
				wsclient = nil
			} else {
				wsclient.SetCloseHandler(func(code int, text string) error {
					wsclient = nil
					return nil
				})
			}
		}
	}()




	go func ()  {
		for {
			out := log.TakeLog()
			
			if wsclient != nil {
				err := wsclient.WriteMessage(websocket.TextMessage,[]byte(out));
				if err != nil {
					wsclient = nil
				}
			}

			if log_file != nil {
				log_file.Write([]byte(fmt.Sprintf("%s\n",out)))
			}
		}
	}()
}




func (daemon *Daemon) HandleDevSim()  {
	go func() {
		for {
			path, err := utils.FindProcessPath("hid", "hid.exe")
			if err != nil {
				panic(err)
			}
			process := exec.Command(path, fmt.Sprintf("--urls=http://localhost:%d", daemon.HID.port))
			id,err := daemon.Childprocess.NewChildProcess(process)
			if err != nil || id == -1{
				log.PushLog("fail to start hid process: %s",err.Error())
			}


			daemon.Childprocess.WaitID(id)
			time.Sleep(1 * time.Second)
		}
	}()
}






func (daemon *Daemon) HandleWebRTC() {
	go func() {
		for {
			time.Sleep(1 * time.Second)
			token, err := api.GetSessionToken(daemon.SessionRegistrationURL, daemon.ServerToken);
			if err != nil {
				log.PushLog("error get session token : %s\n", err.Error())
				continue
			} else if token == "none" {
				daemon.WebRTCHub.sessionToken = "none"
				daemon.WebRTCHub.info = nil
				daemon.WebRTCHub.maxProcCount = 0
				time.Sleep(1 * time.Second)
				continue
			} else if daemon.WebRTCHub.sessionToken != "none" {
				time.Sleep(1 * time.Second)
				continue
			}

			info, err := api.GetSessionInfor(daemon.SessionSettingURL, token)
			if err != nil {
				log.PushLog("error get session infor : %s\n", err.Error())
				continue
			}


			daemon.WebRTCHub.info = info
			daemon.WebRTCHub.sessionToken = token
			daemon.WebRTCHub.maxProcCount = 2
		}
	}()

	go func() {
		for {
			time.Sleep(50 * time.Millisecond)
			if len(daemon.WebRTCHub.procs) > daemon.WebRTCHub.maxProcCount {
				last :=daemon.WebRTCHub.procs[len(daemon.WebRTCHub.procs)-1]
				daemon.Childprocess.CloseID(childprocess.ProcessID(last))
				daemon.WebRTCHub.procs = utils.RemoveElement(&daemon.WebRTCHub.procs,int(last))
			}
		}
	}()

	go func() {
		for {
			if len(daemon.WebRTCHub.procs) >= daemon.WebRTCHub.maxProcCount {
				time.Sleep(50 * time.Millisecond)
				continue
			}



			path, err := utils.FindProcessPath("hub/bin", "hub.exe")
			if err != nil {
				log.PushLog("unable to find hub process %s",err.Error())
				continue;
			}

			session := daemon.WebRTCHub.info;
			process := exec.Command(path,
				"--hid", 		fmt.Sprintf("localhost:%d", daemon.HID.port),
				"--token", 		session.Token,
				"--grpc", 		session.GrpcConf,
				"--webrtc", 	session.WebRTCConf)

			id,err := daemon.Childprocess.NewChildProcess(process)
			if err != nil || id == -1 {
				log.PushLog("error: fail to start hub process: %s",err.Error())
				time.Sleep(1 * time.Second)
				return;
			}

			daemon.WebRTCHub.procs = append(daemon.WebRTCHub.procs, int(id))
			go func ()  {
				daemon.Childprocess.WaitID(id)
				daemon.Childprocess.CloseID(id)

				daemon.WebRTCHub.procs = utils.RemoveElement(&daemon.WebRTCHub.procs,int(id))
			}()
		}
	}()
}

func (daemon *Daemon)TerminateAtTheEnd() {
	go func ()  {
		chann := make(chan os.Signal, 10)
		signal.Notify(chann, syscall.SIGTERM, os.Interrupt)
		<-chann

		daemon.Childprocess.CloseAll()
		time.Sleep(100 * time.Millisecond)
		daemon.Shutdown <- true
	}()
}