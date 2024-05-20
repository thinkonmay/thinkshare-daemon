package httpp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

type GRPCclient struct {
	logger          []string
	worker_info     func() *packet.WorkerInfor
	recv_session    func(*packet.WorkerSession, chan bool) (*packet.WorkerSession, error)
	closed_sesssion func(*packet.WorkerSession) error

	done bool
}

func InitHttppServer() (ret *GRPCclient, err error) {
	ret = &GRPCclient{
		done: false,

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
	ret.wswrapper("new",
		func(conn string, cancel chan bool) ([]byte, error) {
			msg := &packet.WorkerSession{}
			if err := json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}
			if resp, err := ret.recv_session(msg, cancel); err == nil {
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

func (ret *GRPCclient) wswrapper(url string, fun func(content string, cancel chan bool) ([]byte, error)) {
	log.PushLog("registering url handler on %s", url)
	http.HandleFunc("/"+url, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		log.PushLog("incoming request %s", r.URL.Path)
		if !websocket.IsWebSocketUpgrade(r) {
			w.WriteHeader(400)
			w.Write([]byte("request should be made in ws"))
			return
		}

		up := websocket.Upgrader{}
		con, err := up.Upgrade(w, r, nil)
		if err != nil {
			w.WriteHeader(400)
			return
		}

		_, b, err := con.ReadMessage()
		if err != nil {
			return
		}

		cancel := make(chan bool)
		data := make(chan []byte)
		errr := make(chan error)

		fnSend := func(msg string) {
			if err := con.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				cancel <- true
			}
		}

		keepAliveTickler := time.NewTicker(5 * time.Second)
		defer keepAliveTickler.Stop()

		go func() {
			for {
				select {
				case dat := <-data:
					fnSend(string(dat))
					return
				case err := <-errr:
					fnSend(fmt.Sprintf("__ERROR__:%s", err.Error()))
					return
				case <-keepAliveTickler.C:
					fnSend("ping")
				}
			}
		}()

		if resp, err := fun(string(b), cancel); err != nil {
			errr <- err
		} else {
			data <- resp
		}
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
