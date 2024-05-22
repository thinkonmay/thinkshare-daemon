package httpp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

type deployment struct {
	cancel    chan bool
	timestamp time.Time
}
type GRPCclient struct {
	logger          []string
	worker_info     func() *packet.WorkerInfor
	recv_session    func(*packet.WorkerSession, chan bool) (*packet.WorkerSession, error)
	closed_sesssion func(*packet.WorkerSession) error

	pending map[string]*deployment

	mut *sync.Mutex

	done bool
}

func InitHttppServer() (ret *GRPCclient, err error) {
	ret = &GRPCclient{
		done:    false,
		pending: map[string]*deployment{},
		mut:     &sync.Mutex{},

		logger: []string{},
		worker_info: func() *packet.WorkerInfor {
			return &packet.WorkerInfor{}
		},

		recv_session: func(*packet.WorkerSession, chan bool) (*packet.WorkerSession, error) {
			return nil, fmt.Errorf("handler not configured")
		},
		closed_sesssion: func(ws *packet.WorkerSession) error {
			return fmt.Errorf("handler not configured")
		},
	}

	ret.wrapper("ping",
		func(conn string) ([]byte, error) {
			return []byte("pong"), nil
		})
	ret.wrapper("info",
		func(conn string) ([]byte, error) {
			return json.Marshal(ret.worker_info())
		})
	ret.wrapper("log",
		func(conn string) ([]byte, error) {
			return []byte(strings.Join(ret.logger, "\n")), nil
		})
	ret.wrapper("_new",
		func(conn string) ([]byte, error) {
			msg := &packet.WorkerSession{}
			if err := json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}

			ret.mut.Lock()
			deployment, found := ret.pending[msg.Id]
			ret.mut.Unlock()
			if !found {
				return nil, fmt.Errorf("session not found")
			}

			deployment.timestamp = time.Now()
			return []byte("{}"), nil
		})
	ret.wrapper("new",
		func(conn string) ([]byte, error) {
			msg := &packet.WorkerSession{}
			if err := json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}

			deployment := &deployment{
				cancel:    make(chan bool, 8),
				timestamp: time.Now(),
			}
			ret.mut.Lock()
			ret.pending[msg.Id] = deployment
			ret.mut.Unlock()
			running := true
			defer func() {
				running = false
				ret.mut.Lock()
				delete(ret.pending, msg.Id)
				ret.mut.Unlock()
				deployment.cancel <- true
			}()

			go func() {
				for running {
					time.Sleep(time.Second * 10)
					now := time.Now()
					if now.Unix()-deployment.timestamp.Unix() > 9 {
						deployment.cancel <- true
					}
				}
			}()

			if resp, err := ret.recv_session(msg, deployment.cancel); err == nil {
				b, _ := json.Marshal(resp)
				return b, nil
			} else {
				return nil, err
			}
		})
	ret.wrapper("closed",
		func(conn string) ([]byte, error) {
			msg := &packet.WorkerSession{}
			if err = json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}

			return []byte("{}"), ret.closed_sesssion(msg)
		})
	return ret, nil
}

func (ret *GRPCclient) wrapper(url string, fun func(content string) ([]byte, error)) {
	log.PushLog("registering url handler on %s", url)
	http.HandleFunc("/"+url, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		log.PushLog("incoming request %s", r.URL.Path)
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(503)
			w.Write([]byte(err.Error()))
			return
		}

		resp, err := fun(string(b))
		if err != nil {
			log.PushLog("request failed : %s", err.Error())
			w.WriteHeader(503)
			w.Write([]byte(err.Error()))
			return
		}

		w.WriteHeader(200)
		w.Write(resp)
	})
}

func (client *GRPCclient) Stop() {
	client.done = true
}

func (grpc *GRPCclient) Log(source string, level string, log string) {
	grpc.logger = append(grpc.logger, fmt.Sprintf("%s %s %s: %s", time.Now().Format(time.DateTime), source, level, log))
}

func (grpc *GRPCclient) Infor(fun func() *packet.WorkerInfor) {
	grpc.worker_info = fun
}
func (grpc *GRPCclient) RecvSession(fun func(*packet.WorkerSession, chan bool) (*packet.WorkerSession, error)) {
	grpc.recv_session = fun
}
func (grpc *GRPCclient) ClosedSession(fun func(*packet.WorkerSession) error) {
	grpc.closed_sesssion = fun
}
