package httpp

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

type GRPCclient struct {
	host, version, anon_key string
	username                string
	password                string

	logger chan *packet.WorkerLog
	infor  chan *packet.WorkerInfor

	recv_session    chan *packet.WorkerSession
	failed_sesssion chan *packet.WorkerSession
	closed_sesssion chan int

	done bool
}

func InitHttppServer(host string,
	version string,
	anon_key string,
	account credential.Account,
) (ret *GRPCclient, err error) {

	ret = &GRPCclient{
		host:     host,
		version:  version,
		anon_key: anon_key,
		done:     false,

		username: *account.Username,
		password: *account.Password,

		logger:          make(chan *packet.WorkerLog, 8192),
		infor:           make(chan *packet.WorkerInfor, 100),
		failed_sesssion: make(chan *packet.WorkerSession, 100),

		recv_session:    make(chan *packet.WorkerSession, 100),
		closed_sesssion: make(chan int, 100),
	}

	ret.wrapper("ping",
		func(conn string) ([]byte, error) {
			return []byte("ping"), nil
		})
	ret.wrapper("info",
		func(conn string) ([]byte, error) {
			return json.Marshal(<-ret.infor)
		})
	ret.wrapper("log",
		func(conn string) ([]byte, error) {
			time.Sleep(100 * time.Millisecond)
			return json.Marshal(<-ret.logger)
		})
	ret.wrapper("failed",
		func(conn string) ([]byte, error) {
			return json.Marshal(<-ret.failed_sesssion)
		})
	ret.wrapper("new",
		func(conn string) ([]byte, error) {
			msg := &packet.WorkerSession{}
			if err = json.Unmarshal([]byte(conn), msg); err != nil {
				return nil, err
			}

			ret.recv_session <- msg
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

	srv := &http.Server{Addr: ":60000"}
	go func() {
		for {
			srv.ListenAndServe()
		}
	}()
	return ret, nil
}

func (ret *GRPCclient) wrapper(url string, fun func(content string) ([]byte, error)) {
	http.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
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
	if len(grpc.logger) >= 8000 {
		return
	}
	grpc.logger <- &packet.WorkerLog{
		Timestamp: time.Now().Format(time.RFC3339),
		Log:       log,
		Level:     level,
		Source:    source,
	}
}

func (grpc *GRPCclient) Infor(info *packet.WorkerInfor) {
	grpc.infor <- info
}
func (grpc *GRPCclient) RecvSession() *packet.WorkerSession {
	return <-grpc.recv_session
}
func (grpc *GRPCclient) ClosedSession() int {
	return <-grpc.closed_sesssion
}
func (grpc *GRPCclient) FailedSession(session *packet.WorkerSession) {
	grpc.failed_sesssion <- session
}
