package daemon

import "C"
import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thinkonmay/thinkshare-daemon/childprocess"
	"github.com/thinkonmay/thinkshare-daemon/cluster"
	"github.com/thinkonmay/thinkshare-daemon/persistent"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/pocketbase"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/path"
	sharedmemory "github.com/thinkonmay/thinkshare-daemon/utils/shm"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
	"github.com/thinkonmay/thinkshare-daemon/utils/turn"
)

const (
	Httpport = 60000
)

var (
	slow_client = http.Client{Timeout: time.Hour * 24}
)

type internalWorkerSession struct {
	*packet.WorkerSession
	childprocess   []childprocess.ProcessID
	memory_channel *int
}

type Daemon struct {
	packet.WorkerInfor
	memory  *sharedmemory.SharedMemory
	mhandle string
	cleans  []func()

	signaling    *signaling.Signaling
	childprocess *childprocess.ChildProcesses
	cluster      cluster.ClusterConfig
	persist      persistent.Persistent
	turn         *turn.TurnClient

	mutex *sync.Mutex

	session map[string]*internalWorkerSession
	log     int
}

func WebDaemon(persistent persistent.Persistent,
	signaling *signaling.Signaling,
	cluster_path string,
) *Daemon {
	in := (*packet.WorkerInfor)(nil)
	err := (error)(nil)
	for {
		if in, err = system.GetInfor(); err == nil {
			break
		}

		log.PushLog("failed to get info %s", err.Error())
		time.Sleep(time.Second)
	}

	daemon := &Daemon{
		mhandle:     "empty",
		memory:      &sharedmemory.SharedMemory{},
		WorkerInfor: *in,
		mutex:       &sync.Mutex{},
		cleans:      []func(){},
		session:     map[string]*internalWorkerSession{},
		persist:     persistent,
		signaling:   signaling,
		childprocess: childprocess.NewChildProcessSystem(func(proc, log string) {
			fmt.Println(proc + " : " + log)
			persistent.Log(proc, "childprocess", log)
		}),
		log: log.TakeLog(func(log string) {
			persistent.Log("daemon", "infor", log)
		}),
	}

	if daemon.cluster, err = cluster.NewClusterConfig(cluster_path); err != nil {
		log.PushLog("fail to config cluster %s", err.Error())
	}

	if turnconf, ok := daemon.cluster.TurnServer(); ok {
		daemon.turn = &turn.TurnClient{Addr: turnconf.Addr}
	} else {
		log.PushLog("turn config not exist, ignoring turn")
	}

	if daemon.cluster.Pocketbase() {
		pocketbase.StartPocketbase()
	}

	if memory, handle, def, err := sharedmemory.AllocateSharedMemory(); err != nil {
		log.PushLog("fail to create shared memory %s", err.Error())
	} else {
		daemon.mhandle = handle
		daemon.memory = memory
		daemon.cleans = append(daemon.cleans, def)
		sunshine_path, err := path.FindProcessPath("shmsunshine")
		if err != nil {
			log.PushLog("fail to start shmsunshine %s", err.Error())
			return nil
		}

		_, err = daemon.childprocess.NewChildProcess(exec.Command(sunshine_path, handle, fmt.Sprintf("%d", sharedmemory.Audio)))
		if err != nil {
			log.PushLog("fail to start shmsunshine %s", err.Error())
			return nil
		}
	}

	if def := daemon.HandleVirtdaemon(); def != nil {
		daemon.cleans = append(daemon.cleans, def)
	}
	daemon.persist.Infor(daemon.QueryInfo)
	daemon.persist.RecvSession(daemon.handleSession)
	daemon.persist.ClosedSession(daemon.CloseSession)
	daemon.signaling.AuthHandler(daemon.HandleSignaling)
	return daemon
}

func (daemon *Daemon) handleSession(ss *packet.WorkerSession, cancel, keepalive chan bool) (_ *packet.WorkerSession, _err error) {
	process := []childprocess.ProcessID{}
	var channel *int = nil

	err := fmt.Errorf("no session configured")
	if ss.Turn != nil && daemon.turn != nil {
		t := *ss.Turn
		ss.Turn = nil
		if err := daemon.turn.Open(turn.TurnRequest{
			Username: t.Username,
			Password: t.Password,
			PublicIP: *daemon.PublicIP,
			Port:     int(t.Port),
			MaxPort:  int(t.MaxPort),
			MinPort:  int(t.MinPort),
		}); err != nil {
			return nil, err
		}
	}

	if ss.Target != nil {
		return daemon.HandleSessionForward(ss, "new")
	}

	if ss.Display != nil {
		if name, index, err := media.StartVirtualDisplay(
			int(ss.Display.ScreenWidth),
			int(ss.Display.ScreenHeight),
		); err != nil {
			i := ""
			ss.Display.DisplayName = &i
		} else {
			val := int32(index)
			ss.Display.DisplayName, ss.Display.DisplayIndex = &name, &val
		}
	} else if len(media.Displays()) > 0 {
		ss.Display = &packet.DisplaySession{
			DisplayName:  &media.Displays()[0],
			DisplayIndex: nil,
		}
	}

	if ss.Thinkmay != nil {
		process, channel, err = daemon.handleHub(ss)
	}
	if ss.Vm != nil {
		daemon.QueryInfo()
		if ss.Vm.Volumes == nil || len(ss.Vm.Volumes) == 0 {
			if Vm, err := daemon.DeployVM(ss, cancel, keepalive); err != nil {
				return nil, err
			} else {
				ss.Vm = Vm
			}
		} else {
			var session *packet.WorkerSession
			var inf *packet.WorkerInfor
			session, inf, err = daemon.DeployVMwithVolume(ss, cancel, keepalive)
			if err != nil {
				return nil, err
			} else if session != nil {
				return session, nil
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
		WorkerSession:  ss,
		childprocess:   process,
		memory_channel: channel,
	}

	daemon.WorkerInfor.Sessions = append(daemon.WorkerInfor.Sessions, ss)
	return ss, nil
}

func (daemon *Daemon) CloseSession(ss *packet.WorkerSession) error {
	_, err := daemon.HandleSessionForward(ss, "closed")
	if err == nil {
		return nil
	}

	daemon.QueryInfo()
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
	for _, v := range daemon.WorkerInfor.Sessions {
		if ss.Id == v.Id {
			ws = v
		} else {
			wss = append(wss, v)
		}
	}

	daemon.WorkerInfor.Sessions = wss

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
		if iws.Turn != nil && daemon.turn != nil {
			daemon.turn.Close(iws.Turn.Username)
		}
		if iws.memory_channel != nil {
			sharedmemory.SetState(daemon.memory, *iws.memory_channel, 0)
		}
		for _, pi := range iws.childprocess {
			daemon.childprocess.CloseID(pi)
		}
	}

	return nil
}

func (daemon *Daemon) Close() {
	daemon.cluster.Deinit()
	for _, clean := range daemon.cleans {
		clean()
	}
	daemon.childprocess.CloseAll()
	log.RemoveCallback(daemon.log)
	for _, ws := range daemon.WorkerInfor.Sessions {
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
		for _, pi := range ws.childprocess {
			daemon.childprocess.CloseID(pi)
		}
	}
}

func (daemon *Daemon) handleHub(current *packet.WorkerSession) ([]childprocess.ProcessID, *int, error) {
	if daemon.mhandle == "empty" {
		return nil, nil, fmt.Errorf("shared memory not working")
	}
	hub_path, err := path.FindProcessPath("hub")
	if err != nil {
		return nil, nil, err
	}

	var channel int
	if sharedmemory.GetState(daemon.memory, sharedmemory.Video0) == 0 {
		channel = sharedmemory.Video0
	} else if sharedmemory.GetState(daemon.memory, sharedmemory.Video1) == 0 {
		channel = sharedmemory.Video1
	} else {
		return nil, nil, fmt.Errorf("no capture channel available")
	}

	sunshine_path, err := path.FindProcessPath("shmsunshine")
	if err != nil {
		return nil, nil, err
	}

	sharedmemory.SetDisplay(daemon.memory, channel, *current.Display.DisplayName)
	sharedmemory.SetCodec(daemon.memory, channel, 0)
	sunshine, err := daemon.childprocess.NewChildProcess(exec.Command(sunshine_path, daemon.mhandle, fmt.Sprintf("%d", channel)))
	if err != nil {
		return nil, nil, err
	}

	video_token := uuid.NewString()
	audio_token := uuid.NewString()
	cmd := []string{
		"--token", daemon.mhandle,
		"--video_channel", fmt.Sprintf("%d", channel),
		"--stun", current.Thinkmay.StunAddress,
		"--turn", current.Thinkmay.TurnAddress,
		"--turn_username", current.Thinkmay.Username,
		"--turn_password", current.Thinkmay.Password,
		"--video", fmt.Sprintf("http://localhost:%d/handshake/server?token=%s", Httpport, video_token),
		"--audio", fmt.Sprintf("http://localhost:%d/handshake/server?token=%s", Httpport, audio_token),
	}

	video, err := daemon.childprocess.NewChildProcess(exec.Command(hub_path, cmd...))
	if err != nil {
		return nil, nil, err
	}

	current.Thinkmay.AudioToken = &audio_token
	current.Thinkmay.VideoToken = &video_token
	daemon.signaling.AddSignalingChannel(video_token)
	daemon.signaling.AddSignalingChannel(audio_token)
	return []childprocess.ProcessID{video, sunshine}, &channel, nil
}

func (daemon *Daemon) handleSunshine(current *packet.WorkerSession) ([]childprocess.ProcessID, error) {
	hub_path, err := path.FindProcessPath("sunshine")
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

func (daemon *Daemon) HandleSignaling(token string) (*string, bool) {
	for _, s := range daemon.WorkerInfor.Sessions {
		if s.Id == token && s.Vm != nil {
			addr := fmt.Sprintf("http://%s:%d", *s.Vm.PrivateIP, cluster.Httpport)
			return &addr, true
		}
	}

	for _, node := range daemon.cluster.Nodes() {
		sessions, err := node.Sessions()
		if err != nil {
			log.PushLog("ignore signaling session on node %s %s", node.Name(), err.Error())
			continue
		}

		for _, s := range sessions {
			if s.Id == token && s.Vm != nil {
				addr, err := node.RequestBaseURL()
				if err != nil {
					log.PushLog("ignore signaling session on node %s %s", node.Name(), err.Error())
					continue
				}

				return &addr, false
			}

		}
	}

	for _, peer := range daemon.cluster.Peers() {
		if peer.Name() == token {
			addr, err := peer.RequestBaseURL()
			if err != nil {
				log.PushLog("ignore signaling session on node %s %s", peer.Name(), err.Error())
				continue
			}

			return &addr, true
		}
	}

	return nil, false
}

func (daemon *Daemon) QueryInfo() *packet.WorkerInfor {
	info := &daemon.WorkerInfor
	if in, err := system.GetInfor(); err == nil {
		info.Disk = in.Disk
	}

	local := make(chan error)
	jobs := []chan error{local}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				local <- fmt.Errorf("panic occurred in local query: %s", debug.Stack())
			}
		}()
		local <- queryLocal(info)
	}()

	for _, session := range info.Sessions {
		channel := make(chan error)
		jobs = append(jobs, channel)
		go func(s *packet.WorkerSession, c chan error) {
			defer func() {
				if err := recover(); err != nil {
					c <- fmt.Errorf("panic occurred in vm query: %s", debug.Stack())
				}
			}()
			c <- querySession(s)
		}(session, channel)
	}

	for _, peer := range daemon.cluster.Peers() {
		channel := make(chan error)
		jobs = append(jobs, channel)
		go func(s cluster.Peer, c chan error) {
			defer func() {
				if err := recover(); err != nil {
					c <- fmt.Errorf("panic occurred in peer query: %s", debug.Stack())
				}
			}()
			c <- peer.Query()
		}(peer, channel)
	}

	for _, node := range daemon.cluster.Nodes() {
		channel := make(chan error)
		jobs = append(jobs, channel)
		go func(s cluster.Node, c chan error) {
			defer func() {
				if err := recover(); err != nil {
					c <- fmt.Errorf("panic occurred in node query: %s", debug.Stack())
				}
			}()
			c <- node.Query()
		}(node, channel)
	}

	for _, job := range jobs {
		if err := <-job; err != nil {
			log.PushLog("failed to execute job : %s", err.Error())
		}
	}

	return daemon.infoTransform(info)
}

func (daemon *Daemon) infoTransform(inf *packet.WorkerInfor) *packet.WorkerInfor {
	cp := *inf
	for _, node := range daemon.cluster.Nodes() {
		ss, err := node.Sessions()
		if err != nil {
			log.PushLog("ignore info from node %s %s", node.Name(), err.Error())
			continue
		}
		gpus, err := node.GPUs()
		if err != nil {
			log.PushLog("ignore info from node %s %s", node.Name(), err.Error())
			continue
		}
		volumes, err := node.Volumes()
		if err != nil {
			log.PushLog("ignore info from node %s %s", node.Name(), err.Error())
			continue
		}

		cp.Sessions = append(cp.Sessions, ss...)
		cp.GPUs = append(cp.GPUs, gpus...)
		cp.Volumes = append(cp.Volumes, volumes...)
	}

	for _, node := range daemon.cluster.Peers() {
		node.Query()
		if info, err := node.Info(); err != nil {
			log.PushLog("failed to query info %s", err.Error())
		} else {
			cp.Peers = append(cp.Peers, info)
		}
	}

	return &cp
}

func (daemon *Daemon) HandleSessionForward(ss *packet.WorkerSession, command string) (*packet.WorkerSession, error) {
	if ss.Target == nil {
		for _, node := range daemon.cluster.Nodes() {
			sessions, err := node.Sessions()
			if err != nil {
				log.PushLog("ignore session fwd on node %s %s", node.Name(), err.Error())
				continue
			}

			for _, session := range sessions {
				if session.Id != ss.Id {
					continue
				}

				log.PushLog("forwarding command %s to node %s", command, node.Name())

				b, _ := json.Marshal(ss)

				url, err := node.RequestBaseURL()
				if err != nil {
					log.PushLog("ignore session fwd on node %s %s", node.Name(), err.Error())
					continue
				}

				resp, err := slow_client.Post(
					fmt.Sprintf("%s/%s", url, command),
					"application/json",
					strings.NewReader(string(b)))
				if err != nil {
					log.PushLog("failed to request %s", err.Error())
					continue
				}

				b, err = io.ReadAll(resp.Body)
				if err != nil {
					log.PushLog(err.Error())
					continue
				}
				if resp.StatusCode != 200 {
					log.PushLog("failed to request %s", string(b))
					continue
				}

				nss := packet.WorkerSession{}
				err = json.Unmarshal(b, &nss)
				if err != nil {
					log.PushLog("failed to request %s", err.Error())
					continue
				}

				return &nss, nil
			}
		}

		for _, peer := range daemon.cluster.Peers() {
			info, err := peer.Info()
			if err != nil {
				log.PushLog("ignore session fwd on peer %s %s", peer.Name(), err.Error())
				continue
			}

			for _, session := range info.Sessions {
				if session.Id != ss.Id {
					continue
				}

				log.PushLog("forwarding command %s to peer %s", command, peer.Name())

				b, _ := json.Marshal(ss)

				url, err := peer.RequestBaseURL()
				if err != nil {
					log.PushLog("ignore session fwd on peer %s %s", peer.Name(), err.Error())
					continue
				}

				resp, err := slow_client.Post(
					fmt.Sprintf("%s/%s", url, command),
					"application/json",
					strings.NewReader(string(b)))
				if err != nil {
					log.PushLog("failed to request %s", err.Error())
					continue
				}

				b, err = io.ReadAll(resp.Body)
				if err != nil {
					log.PushLog(err.Error())
					continue
				}
				if resp.StatusCode != 200 {
					log.PushLog("failed to request %s", string(b))
					continue
				}

				nss := packet.WorkerSession{}
				err = json.Unmarshal(b, &nss)
				if err != nil {
					log.PushLog("failed to request %s", err.Error())
					continue
				}

				return &nss, nil
			}
		}

		return nil, fmt.Errorf("no session found on any node")
	}

	for _, session := range daemon.WorkerInfor.Sessions {
		if session == nil ||
			ss.Target == nil ||
			session.Id != *ss.Target ||
			session.Vm == nil ||
			session.Vm.PrivateIP == nil {
			continue
		}

		log.PushLog("forwarding command %s to vm %s", command, *session.Vm.PrivateIP)

		nss := *ss
		nss.Target = nil
		b, _ := json.Marshal(nss)

		resp := (*http.Response)(nil)
		err := (error)(nil)
		count := 0
		for count < 5 {
			resp, err = slow_client.Post(
				fmt.Sprintf("http://%s:%d/%s", *session.Vm.PrivateIP, Httpport, command),
				"application/json",
				strings.NewReader(string(b)))
			if err != nil {
				log.PushLog("failed to request %s", err.Error())
				time.Sleep(time.Second * 5)
				count++
				continue
			} else {
				break
			}
		}

		if count == 5 {
			log.PushLog("giving up doing request %s to VM %s", command, *session.Vm.PrivateIP)
			continue
		}

		b, err = io.ReadAll(resp.Body)
		if err != nil {
			log.PushLog("failed to parse request %s", err.Error())
			continue
		}
		if resp.StatusCode != 200 {
			log.PushLog("failed to request %s", string(b))
			continue
		}

		worker_session := packet.WorkerSession{}
		err = json.Unmarshal(b, &worker_session)
		if err != nil {
			log.PushLog("failed to request %s", err.Error())
			continue
		}

		return &worker_session, nil
	}

	for _, node := range daemon.cluster.Nodes() {
		sessions, err := node.Sessions()
		if err != nil {
			log.PushLog("ignore session fwd on node %s %s", node.Name(), err.Error())
			return nil, err
		}

		for _, session := range sessions {
			if session == nil ||
				session.Id != *ss.Target ||
				session.Vm == nil ||
				session.Vm.PrivateIP == nil {
				continue
			}

			log.PushLog("forwarding command %s to node %s, vm %s", command, node.Name(), *session.Vm.PrivateIP)

			b, _ := json.Marshal(ss)

			url, err := node.RequestBaseURL()
			if err != nil {
				log.PushLog("ignore session fwd on node %s %s", node.Name(), err.Error())
				continue
			}

			resp, err := slow_client.Post(
				fmt.Sprintf("%s/%s", url, command),
				"application/json",
				strings.NewReader(string(b)))
			if err != nil {
				log.PushLog("failed to request %s", err.Error())
				continue
			}

			b, err = io.ReadAll(resp.Body)
			if err != nil {
				log.PushLog("failed to parse request %s", err.Error())
				continue
			}
			if resp.StatusCode != 200 {
				log.PushLog("failed to request %s", string(b))
				continue
			}

			nss := packet.WorkerSession{}
			err = json.Unmarshal(b, &nss)
			if err != nil {
				log.PushLog("failed to request %s", err.Error())
				continue
			}

			return &nss, nil
		}
	}

	for _, peer := range daemon.cluster.Peers() {
		if peer.Name() != *ss.Target {
			continue
		}

		log.PushLog("forwarding command %s to peer %s", command, peer.Name())
		nss := *ss
		nss.Target = nil
		b, _ := json.Marshal(nss)

		url, err := peer.RequestBaseURL()
		if err != nil {
			log.PushLog("ignore session fwd on node %s %s", peer.Name(), err.Error())
			continue
		}

		resp, err := slow_client.Post(
			fmt.Sprintf("%s/%s", url, command),
			"application/json",
			strings.NewReader(string(b)))
		if err != nil {
			log.PushLog("failed to request %s", err.Error())
			continue
		}

		b, err = io.ReadAll(resp.Body)
		if err != nil {
			log.PushLog("failed to parse request %s", err.Error())
			continue
		}
		if resp.StatusCode != 200 {
			log.PushLog("failed to request %s", string(b))
			continue
		}

		nss = packet.WorkerSession{}
		err = json.Unmarshal(b, &nss)
		if err != nil {
			log.PushLog("failed to request %s", err.Error())
			continue
		}

		return &nss, nil
	}

	return nil, fmt.Errorf("no receiver detected")
}
