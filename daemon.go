package daemon

import (
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thinkonmay/thinkshare-daemon/childprocess"
	"github.com/thinkonmay/thinkshare-daemon/persistent"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/path"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
	"github.com/thinkonmay/thinkshare-daemon/utils/turn"
)

type internalWorkerSession struct {
	childprocess []childprocess.ProcessID
	turn_server  *turn.TurnServer
}

type Daemon struct {
	info packet.WorkerInfor

	signaling    *signaling.Signaling
	childprocess *childprocess.ChildProcesses
	persist      persistent.Persistent

	mutex *sync.Mutex

	session map[string]*internalWorkerSession
	log     int
}

func WebDaemon(persistent persistent.Persistent,
	signaling *signaling.Signaling,
	cluster *ClusterConfig,
) *Daemon {
	i, err := system.GetInfor()
	if err != nil {
		log.PushLog("failed to get info %s", err.Error())
		time.Sleep(time.Second)
		return WebDaemon(persistent, signaling, cluster)
	}

	daemon := &Daemon{
		info:      *i,
		mutex:     &sync.Mutex{},
		session:   map[string]*internalWorkerSession{},
		persist:   persistent,
		signaling: signaling,
		childprocess: childprocess.NewChildProcessSystem(func(proc, log string) {
			fmt.Println(proc + " : " + log)
			persistent.Log(proc, "childprocess", log)
		}),
		log: log.TakeLog(func(log string) {
			persistent.Log("daemon.exe", "infor", log)
		}),
	}

	go daemon.HandleVirtdaemon(cluster)
	daemon.persist.Infor(func() *packet.WorkerInfor {
		QueryInfo(&daemon.info)
		result := InfoBuilder(daemon.info)
		return &result
	})

	daemon.persist.RecvSession(func(ss *packet.WorkerSession) (*packet.WorkerSession, error) {

		process := []childprocess.ProcessID{}
		var t *turn.TurnServer = nil

		err := fmt.Errorf("no session configured")
		if ss.Turn != nil {
			t, err = turn.Open(
				ss.Turn.Username,
				ss.Turn.Password,
				int(ss.Turn.MinPort),
				int(ss.Turn.MaxPort),
				int(ss.Turn.Port),
			)
		}

		if ss.Target != nil {
			return daemon.HandleSessionForward(ss, "new")
		}

		if ss.Display != nil {
			name, index, err := media.StartVirtualDisplay(
				int(ss.Display.ScreenWidth),
				int(ss.Display.ScreenHeight),
			)
			if err != nil {
				return nil, err
			}
			val := int32(index)
			ss.Display.DisplayName, ss.Display.DisplayIndex = &name, &val
		} else if len(media.Displays()) > 0 {
			ss.Display = &packet.DisplaySession{
				DisplayName:  &media.Displays()[0],
				DisplayIndex: nil,
			}
		}

		if ss.Thinkmay != nil {
			process, err = daemon.handleHub(ss)
		}
		if ss.Vm != nil {
			if ss.Vm.Volumes == nil || len(ss.Vm.Volumes) == 0 {
				var Vm *packet.WorkerInfor
				Vm, err = daemon.DeployVM(ss)
				if err != nil {
					if err.Error() == "ran out of gpu" {
						return daemon.DeployVMonNode(ss)
					}
				} else {
					ss.Vm = Vm
				}
			} else {
				var session *packet.WorkerSession
				var inf *packet.WorkerInfor
				session,inf,err = daemon.DeployVMwithVolume(ss)
				if err != nil {
					return nil, err
				} else if session != nil {
					return session,nil
				} else if inf != nil {
					ss.Vm = inf
				}
			}
		}
		if ss.Sunshine != nil {
			process, err = daemon.handleSunshine(ss)
		}

		if err != nil {
			log.PushLog("session failed %s", err.Error())
			return nil, err
		}

		log.PushLog("session creation successful")
		daemon.session[ss.Id] = &internalWorkerSession{
			turn_server:  t,
			childprocess: process,
		}

		daemon.info.Sessions = append(daemon.info.Sessions, ss)
		return ss, nil
	})

	go func() {
		for {
			ss := daemon.persist.ClosedSession()
			_, err := daemon.HandleSessionForward(ss, "closed")
			if err == nil {
				continue
			}

			log.PushLog("terminating session %s", ss)
			keys := make([]string, 0, len(daemon.session))
			for k, _ := range daemon.session {
				keys = append(keys, k)
			}

			var ws *packet.WorkerSession = nil
			var iws *internalWorkerSession = nil
			for _, v := range keys {
				if ss.Id == v {
					iws = daemon.session[v]
					delete(daemon.session, v)
				}
			}

			wss := []*packet.WorkerSession{}
			for _, v := range daemon.info.Sessions {
				if ss.Id == v.Id {
					ws = v
				} else {
					wss = append(wss, v)
				}
			}

			daemon.info.Sessions = wss

			if ws != nil {
				if ws.Display != nil {
					if ws.Display.DisplayIndex != nil {
						media.RemoveVirtualDisplay(int(*ws.Display.DisplayIndex))
					}
				}
				if ws.Vm != nil {
					daemon.ShutdownVM(ws.Vm)
				}
				if ws.Thinkmay != nil {
					daemon.signaling.RemoveSignalingChannel(*ws.Thinkmay.VideoToken)
					daemon.signaling.RemoveSignalingChannel(*ws.Thinkmay.AudioToken)
				}
			}
			if iws != nil {
				if iws.turn_server != nil {
					iws.turn_server.Close()
				}
				for _, pi := range iws.childprocess {
					daemon.childprocess.CloseID(pi)
				}
			}
		}
	}()

	daemon.signaling.AuthHandler(daemon.HandleSignaling)

	return daemon
}

func (daemon *Daemon) Close() {
	deinit()
	daemon.childprocess.CloseAll()
	log.RemoveCallback(daemon.log)
	for _, ws := range daemon.info.Sessions {
		if ws.Display != nil {
			if ws.Display.DisplayIndex != nil {
				media.RemoveVirtualDisplay(int(*ws.Display.DisplayIndex))
			}
		}
		if ws.Vm != nil {
			daemon.ShutdownVM(ws.Vm)
		}
	}

	for _, ws := range daemon.session {
		if ws.turn_server != nil {
			ws.turn_server.Close()
		}
		for _, pi := range ws.childprocess {
			daemon.childprocess.CloseID(pi)
		}
	}
}

func (daemon *Daemon) handleHub(current *packet.WorkerSession) ([]childprocess.ProcessID, error) {
	hub_path, err := path.FindProcessPath("", "hub.exe")
	if err != nil {
		return nil, err
	}

	video_token := uuid.NewString()
	audio_token := uuid.NewString()
	cmd := []string{
		"--stun", current.Thinkmay.StunAddress,
		"--turn", current.Thinkmay.TurnAddress,
		"--turn_username", current.Thinkmay.Username,
		"--turn_password", current.Thinkmay.Password,
		"--display", *current.Display.DisplayName,
		"--video", fmt.Sprintf("http://localhost:60000/handshake/server?token=%s", video_token),
		"--audio", fmt.Sprintf("http://localhost:60000/handshake/server?token=%s", audio_token),
	}

	video, err := daemon.childprocess.NewChildProcess(exec.Command(hub_path, cmd...))
	if err != nil {
		return nil, err
	}

	current.Thinkmay.AudioToken = &audio_token
	current.Thinkmay.VideoToken = &video_token
	daemon.signaling.AddSignalingChannel(video_token)
	daemon.signaling.AddSignalingChannel(audio_token)
	return []childprocess.ProcessID{video}, nil
}

func (daemon *Daemon) handleSunshine(current *packet.WorkerSession) ([]childprocess.ProcessID, error) {
	hub_path, err := path.FindProcessPath("", "sunshine.exe")
	if err != nil {
		return nil, err
	}

	cmd := []string{
		"--username", current.Sunshine.Username,
		"--password", current.Sunshine.Password,
		"--port", current.Sunshine.Port,
	}

	id, err := daemon.childprocess.NewChildProcess(exec.Command(hub_path, cmd...))
	if err != nil {
		return nil, err
	}

	return []childprocess.ProcessID{id}, nil
}
