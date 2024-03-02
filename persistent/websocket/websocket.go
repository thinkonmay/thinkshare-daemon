package websocket

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
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

func InitGRPCClient(host string,
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

	ret.wrapper("ping", func(conn *websocket.Conn) error {
		time.Sleep(5 * time.Second)
		return conn.WriteMessage(websocket.TextMessage, []byte("ping"))
	})
	ret.wrapper("info", func(conn *websocket.Conn) error {
		return conn.WriteJSON(<-ret.infor)
	})
	ret.wrapper("log", func(conn *websocket.Conn) error {
		time.Sleep(100 * time.Millisecond)
		return conn.WriteJSON(<-ret.logger)
	})
	ret.wrapper("failed", func(conn *websocket.Conn) error {
		return conn.WriteJSON(<-ret.failed_sesssion)
	})
	ret.wrapper("new", func(conn *websocket.Conn) error {
		msg := &packet.WorkerSession{}
		if err = conn.ReadJSON(msg); err != nil {
			return err
		}

		ret.recv_session <- msg
		return nil
	})
	ret.wrapper("closed", func(conn *websocket.Conn) error {
		msg := &struct {
			Id int `json:"id"`
		}{Id: 0}
		if err = conn.ReadJSON(msg); err != nil {
			return err
		}

		ret.closed_sesssion <- msg.Id
		return nil
	})
	return ret, nil
}

func (ret *GRPCclient) wrapper(url string, fun func(conn *websocket.Conn) error) {
	connect := func() *websocket.Conn {
		if ret.done {
			return nil
		}

		dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
		dial_ctx, _ := context.WithTimeout(context.TODO(), 10*time.Second)
		conn, _, err := dialer.DialContext(dial_ctx,
			fmt.Sprintf("wss://%s/functions/%s/%s",ret.host,ret.version,url),
			http.Header{
				"username": []string{ret.username},
				"password": []string{ret.password},
			})

		if err != nil {
			log.PushLog("failed to dial %s : %s", url , err.Error())
			time.Sleep(100 * time.Millisecond)
			return nil
		}

		return conn
	}

	go func() {
		for {
			if ret.done {
				return
			}

			conn := connect()
			if conn == nil {
				continue
			}

			for {
				err := fun(conn)
				if err != nil {
					log.PushLog("error receive channel %s from conductor %s", url, err.Error())
					break
				}
			}
		}
	}()
}

func (client *GRPCclient) Stop() {
	client.done = true
}

func (grpc *GRPCclient) Log(source string, level string, log string) {
	if len(grpc.logger) >= 8000 { return }
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
