package httpp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

type GRPCclient struct {
	logger      []string
	worker_info func() *packet.WorkerInfor
	recv_session    func(*packet.WorkerSession) (*packet.WorkerSession, error)
	closed_sesssion chan string

	done bool
}

func InitHttppServer() (ret *GRPCclient, err error) {
	ret = &GRPCclient{
		done: false,

		logger: []string{},
		worker_info: func() *packet.WorkerInfor {
			return &packet.WorkerInfor{}
		},

		recv_session: func(ws *packet.WorkerSession) (*packet.WorkerSession, error) {
			return nil, fmt.Errorf("handler not configured")
		},
		closed_sesssion: make(chan string),
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
			return []byte(strings.Join(ret.logger, "\n")),nil
		})
	ret.wrapper("new",
		func(conn string) ([]byte, error) {
			msg := &packet.WorkerSession{}
			if err := json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}
			if resp, err := ret.recv_session(msg); err == nil {
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

			ret.closed_sesssion <- msg.Id
			return []byte("ok"), nil
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
	grpc.logger = append(grpc.logger, fmt.Sprintf("%s %s %s: %s",time.Now().Format(time.DateTime), source, level, log))
}

func (grpc *GRPCclient) Infor(fun func() *packet.WorkerInfor) {
	grpc.worker_info = fun
}
func (grpc *GRPCclient) RecvSession(fun func(*packet.WorkerSession) (*packet.WorkerSession, error)) {
	grpc.recv_session = fun
}
func (grpc *GRPCclient) ClosedSession() string {
	return <-grpc.closed_sesssion
}
