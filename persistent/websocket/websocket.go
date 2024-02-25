package websocket

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)


type GRPCclient struct {
	username string
	password string

	logger     chan *packet.WorkerLog
	infor      chan *packet.WorkerInfor

	state_out chan *packet.WorkerSession
	state_in  chan *[]packet.WorkerSession

	done      bool
}

func InitGRPCClient(host string,
					version string,
					anon_key string,
					account credential.Account,
					) (ret *GRPCclient, err error) {

	ret = &GRPCclient{
		done:      false,

		username : *account.Username,
		password : *account.Password,

		logger     : make(chan *packet.WorkerLog,100),
		infor      : make(chan *packet.WorkerInfor,100),

		state_out : make(chan *packet.WorkerSession,100),
		state_in  : make(chan *[]packet.WorkerSession,100),
	}


	connect := func(url string) *websocket.Conn{
		if ret.done {
			return nil
		}


		dialer := websocket.Dialer{ HandshakeTimeout: 10 * time.Second, }
		dial_ctx,_ := context.WithTimeout(context.TODO(),10 * time.Second)
		conn, _, err := dialer.DialContext(dial_ctx,
			fmt.Sprintf("wss://%s/api/persistent/%s/%s",host,version,url),
			http.Header{
				"username" : []string{*account.Username},
				"password" : []string{*account.Password},
			})


		if err != nil {
			log.PushLog("failed to dial : %s",err.Error())
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

			conn := connect("log")
			if conn == nil {
				continue
			}

			for {
				msg := <-ret.logger
				if ret.done {
					break
				}

				if err := conn.WriteJSON(msg); err != nil && err != io.EOF && !strings.Contains(err.Error(), "error while marshaling"){
					log.PushLog("error sending log to conductor %s", err.Error())
					ret.logger <- msg
					break
				}
			}
		}
	}()

	go func() {
		for {
			if ret.done {
				return
			} 

			conn := connect("info")
			if conn == nil {
				continue
			}

			for {
				msg := <-ret.infor
				if err := conn.WriteJSON(msg); err != nil && err != io.EOF{
					log.PushLog("error sending hwinfor to conductor %s", err.Error())
					ret.infor <- msg
					break
				}
			}
		}
	}()
	go func() {
		for {
			if ret.done {
				return
			} 

			conn := connect("sync")
			if conn == nil {
				continue
			}

			done := make(chan bool, 2)
			go func() {
				for {
					msg :=<- ret.state_in
					if conn == nil {
						done <- true
						break
					}
					if err := conn.WriteJSON(msg); err != nil {
						log.PushLog("error sending session state to conductor %s", err.Error())
						ret.state_in <- msg
						done <- true
						break
					}
				}
			}()
			go func() {
				for {
					msg := &packet.WorkerSession{}
					if conn == nil {
						done <- true
						break
					}
					if err = conn.ReadJSON(msg); err != nil {
						log.PushLog("error receive session state from conductor %s", err.Error())
						done <- true
						break
					}
					ret.state_out <- msg
				}
			}()
			<-done
		}
	}()
	return ret, nil
}



func (client *GRPCclient) Stop() {
	client.done = true
}

func (grpc *GRPCclient) Log(source string, level string, log string) {
	grpc.logger <- &packet.WorkerLog{
		Timestamp: time.Now().Format(time.RFC3339),
		Log:       log,
		Level:     level,
		Source:    source,
	}
}

func (grpc *GRPCclient) Infor(log *packet.WorkerInfor) {
	grpc.infor <- log
}
func (grpc *GRPCclient) RecvSession() packet.WorkerSession {
	return *<-grpc.state_out
}
func (grpc *GRPCclient) SyncSession(log []packet.WorkerSession) {
	grpc.state_in <-&log
}