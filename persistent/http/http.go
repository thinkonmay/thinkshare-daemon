package httpp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

type GRPCclient struct {
	account_id string

	logger []*packet.WorkerLog
	infor  chan *packet.WorkerInfor

	recv_session    func(*packet.WorkerSession) error
	closed_sesssion chan int

	done bool
}

func InitHttppServer(account_id string) (ret *GRPCclient, err error) {
	ret = &GRPCclient{
		done:       false,
		account_id: account_id,

		logger: []*packet.WorkerLog{},
		infor:  make(chan *packet.WorkerInfor, 100),

		recv_session:    func(ws *packet.WorkerSession) error { return fmt.Errorf("handler not configured") },
		closed_sesssion: make(chan int, 100),
	}

	ret.wrapper("ping",
		func(conn string) ([]byte, error) {
			return []byte("pong"), nil
		})
	ret.wrapper("info",
		func(conn string) ([]byte, error) {
			return json.Marshal(<-ret.infor)
		})
	ret.wrapper("log",
		func(conn string) ([]byte, error) {
			time.Sleep(100 * time.Millisecond)
			return json.Marshal(ret.logger)
		})
	ret.wrapper("new",
		func(conn string) ([]byte, error) {
			msg := &packet.WorkerSession{}
			if err := json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}
			if err = ret.recv_session(msg); err != nil {
				return nil, err
			}

			return []byte("ok"), nil
		})
	ret.wrapper("closed",
		func(conn string) ([]byte, error) {
			msg := &struct {
				Id int `json:"id"`
			}{Id: 0}
			if err = json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}

			ret.closed_sesssion <- msg.Id
			return []byte("ok"), nil
		})

	return ret, nil
}

func (ret *GRPCclient) wrapper(url string, fun func(content string) ([]byte, error)) {
	http.HandleFunc("/"+url, func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("incoming")
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
	grpc.logger = append(grpc.logger, &packet.WorkerLog{
		Timestamp: time.Now().Format(time.RFC3339),
		Log:       log,
		Level:     level,
		Source:    source,
	})
}

func (grpc *GRPCclient) Infor(info *packet.WorkerInfor) {
	grpc.infor <- info
}
func (grpc *GRPCclient) RecvSession(fun func(*packet.WorkerSession) error) {
	grpc.recv_session = fun
}
func (grpc *GRPCclient) ClosedSession() int {
	return <-grpc.closed_sesssion
}
