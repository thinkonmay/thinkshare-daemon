package daemon

/*
#include "smemory.h"
#include <string.h>
*/
import "C"
import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/google/uuid"
	"github.com/thinkonmay/thinkshare-daemon/childprocess"
	"github.com/thinkonmay/thinkshare-daemon/cluster"
	"github.com/thinkonmay/thinkshare-daemon/persistent"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/pocketbase"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/path"
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
	childprocess   []childprocess.ProcessID
	turn_server    *turn.TurnServer
	memory_channel *int
}

type Daemon struct {
	packet.WorkerInfor
	memory  *SharedMemory
	mhandle string
	cleans  []func()

	signaling    *signaling.Signaling
	childprocess *childprocess.ChildProcesses
	cluster      cluster.ClusterConfig
	persist      persistent.Persistent

	mutex *sync.Mutex

	session map[string]*internalWorkerSession
	log     int
}

func WebDaemon(persistent persistent.Persistent,
	signaling *signaling.Signaling,
	cluster_path, web_path string,
) *Daemon {
	i := (*packet.WorkerInfor)(nil)
	err := (error)(nil)
	for {
		if i, err = system.GetInfor(); err == nil {
			break
		}

		log.PushLog("failed to get info %s", err.Error())
		time.Sleep(time.Second)
	}

	daemon := &Daemon{
		mhandle:     "empty",
		memory:      &SharedMemory{},
		WorkerInfor: *i,
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

	if domain := daemon.cluster.Domain(); domain != nil {
		pocketbase.StartPocketbase(web_path, []string{*domain})
	}

	if memory, handle, def, err := AllocateSharedMemory(); err != nil {
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

		_, err = daemon.childprocess.NewChildProcess(exec.Command(sunshine_path, handle, fmt.Sprintf("%d", Audio)))
		if err != nil {
			log.PushLog("fail to start shmsunshine %s", err.Error())
			return nil
		}
	}

	def := daemon.HandleVirtdaemon()
	daemon.cleans = append(daemon.cleans, def)
	daemon.persist.Infor(func() *packet.WorkerInfor {
		result := daemon.QueryInfo(&daemon.WorkerInfor)
		return &result
	})

	daemon.persist.RecvSession(func(ss *packet.WorkerSession, cancel chan bool) (*packet.WorkerSession, error) {

		process := []childprocess.ProcessID{}
		var t *turn.TurnServer = nil
		var channel *int = nil

		err := fmt.Errorf("no session configured")
		if ss.Turn != nil {
			if t, err = turn.Open(
				ss.Turn.Username,
				ss.Turn.Password,
				int(ss.Turn.MinPort),
				int(ss.Turn.MaxPort),
				int(ss.Turn.Port),
			); err != nil {
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
			daemon.QueryInfo(&daemon.WorkerInfor)
			if ss.Vm.Volumes == nil || len(ss.Vm.Volumes) == 0 {
				if Vm, err := daemon.DeployVM(ss, cancel); err != nil {
					return nil, err
				} else {
					ss.Vm = Vm
				}
			} else {
				var session *packet.WorkerSession
				var inf *packet.WorkerInfor
				session, inf, err = daemon.DeployVMwithVolume(ss, cancel)
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
			turn_server:    t,
			childprocess:   process,
			memory_channel: channel,
		}

		daemon.WorkerInfor.Sessions = append(daemon.WorkerInfor.Sessions, ss)
		return ss, nil
	})

	daemon.persist.ClosedSession(func(ss *packet.WorkerSession) error {
		_, err := daemon.HandleSessionForward(ss, "closed")
		if err == nil {
			return nil
		}

		daemon.QueryInfo(&daemon.WorkerInfor)
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
			if iws.turn_server != nil {
				iws.turn_server.Close()
			}
			if iws.memory_channel != nil {
				daemon.memory.queues[*iws.memory_channel].metadata.active = 0
			}
			for _, pi := range iws.childprocess {
				daemon.childprocess.CloseID(pi)
			}
		}

		return nil
	})

	daemon.signaling.AuthHandler(daemon.HandleSignaling)

	return daemon
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
		if ws.turn_server != nil {
			ws.turn_server.Close()
		}
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
	if daemon.memory.queues[Video0].metadata.active == 0 {
		channel = Video0
	} else if daemon.memory.queues[Video1].metadata.active == 0 {
		channel = Video1
	} else {
		return nil, nil, fmt.Errorf("no capture channel available")
	}

	sunshine_path, err := path.FindProcessPath("shmsunshine")
	if err != nil {
		return nil, nil, err
	}

	display := []byte(*current.Display.DisplayName)
	if len(display) > 0 {
		memcpy(unsafe.Pointer(&daemon.memory.queues[channel].metadata.display[0]), unsafe.Pointer(&display[0]), len(display))
	}
	daemon.memory.queues[channel].metadata.codec = 0
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
		sessions, err := peer.Sessions()
		if err != nil {
			log.PushLog("ignore signaling session on node %s %s", peer.Name(), err.Error())
			continue
		}

		for _, s := range sessions {
			if s.Id == token && s.Vm != nil {
				addr, err := peer.RequestBaseURL()
				if err != nil {
					log.PushLog("ignore signaling session on node %s %s", peer.Name(), err.Error())
					continue
				}

				return &addr, false
			}

		}
	}

	return nil, false
}

func (daemon *Daemon) QueryInfo(info *packet.WorkerInfor) packet.WorkerInfor {
	local := make(chan error)
	jobs := []chan error{local}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				local <- fmt.Errorf("panic occurred: %v", err)
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
					c <- fmt.Errorf("panic occurred: %v", err)
				}
			}()
			c <- querySession(s)
		}(session, channel)
	}

	for _, peer := range daemon.cluster.Peers() {
		channel := make(chan error)
		jobs = append(jobs, channel)
		go func(s cluster.Node, c chan error) {
			defer func() {
				if err := recover(); err != nil {
					c <- fmt.Errorf("panic occurred: %v", err)
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
					c <- fmt.Errorf("panic occurred: %v", err)
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

	return daemon.infoBuilder(*info)
}

func (daemon *Daemon) infoBuilder(cp packet.WorkerInfor) packet.WorkerInfor {
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

	return cp
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
			sessions, err := peer.Sessions()
			if err != nil {
				log.PushLog("ignore session fwd on peer %s %s", peer.Name(), err.Error())
				continue
			}

			for _, session := range sessions {
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
		resp, err := slow_client.Post(
			fmt.Sprintf("http://%s:%d/%s", *session.Vm.PrivateIP, Httpport, command),
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
		sessions, err := peer.Sessions()
		if err != nil {
			log.PushLog("ignore session fwd on node %s %s", peer.Name(), err.Error())
			return nil, err
		}

		for _, session := range sessions {
			if session == nil ||
				session.Id != *ss.Target ||
				session.Vm == nil ||
				session.Vm.PrivateIP == nil {
				continue
			}

			log.PushLog("forwarding command %s to node %s, vm %s", command, peer.Name(), *session.Vm.PrivateIP)

			b, _ := json.Marshal(ss)

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

			nss := packet.WorkerSession{}
			err = json.Unmarshal(b, &nss)
			if err != nil {
				log.PushLog("failed to request %s", err.Error())
				continue
			}

			return &nss, nil
		}
	}

	return nil, fmt.Errorf("no receiver detected")
}
