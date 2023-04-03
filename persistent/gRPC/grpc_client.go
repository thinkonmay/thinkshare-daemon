package grpc

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/thinkonmay/conductor/protocol/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type GRPCclient struct {
	conn   *grpc.ClientConn
	stream packet.ConductorClient

	username string
	password string

	logger     chan *packet.WorkerLog
	monitoring chan *packet.WorkerMetric
	infor      chan *packet.WorkerInfor
	devices    chan *packet.MediaDevice

	state_out chan *packet.WorkerSessions
	state_in  chan *packet.WorkerSessions

	done      bool
	connected bool
}

func InitGRPCClient(host string,
					port int,
					account credential.Account,
					) (ret *GRPCclient, err error) {
	conn, err := grpc.Dial(
		fmt.Sprintf("%s:%d", host, port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return nil, err
	}

	ret = &GRPCclient{
		conn:  conn,
		stream : packet.NewConductorClient(conn),
		connected: true,
		done:      false,

		username : account.Username,
		password : account.Password,

		logger     : make(chan *packet.WorkerLog,100),
		monitoring : make(chan *packet.WorkerMetric,100),
		infor      : make(chan *packet.WorkerInfor,100),
		devices    : make(chan *packet.MediaDevice,100),

		state_out : make(chan *packet.WorkerSessions,100),
		state_in  : make(chan *packet.WorkerSessions,100),
	}


	go func() {
		for {
			if ret.done {
				return
			}
			if ret.stream == nil {
				time.Sleep(2 * time.Second)
				continue
			}

			client, err := ret.stream.Logger(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				continue
			}

			for {
				if err := client.Send(<-ret.logger); err != nil {
					log.PushLog("error sending log to conductor %s", err.Error())
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
			client, err := ret.stream.Monitor(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				continue
			}

			for {
				if err := client.Send(<-ret.monitoring); err != nil {
					log.PushLog("error sending metric to conductor %s", err.Error())
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
			client, err := ret.stream.Mediadevice(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				continue
			}

			for {
				dv := <-ret.devices
				if err := client.Send(dv); err != nil {
					log.PushLog("error sync media device : %s", err.Error())
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
			client, err := ret.stream.Infor(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				continue
			}

			for {
				if err := client.Send(<-ret.infor); err != nil {
					log.PushLog("error sending hwinfor to conductor %s", err.Error())
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

			ctx, cancel := context.WithCancel(ret.genContext())
			client, err := ret.stream.Sync(ctx)
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				continue
			}

			done := make(chan bool, 2)
			go func() {
				for {
					if err := client.Send(<-ret.state_in); err != nil && err != io.EOF{
						log.PushLog("error sending session state to conductor %s", err.Error())
						done <- true
						break
					}
				}
			}()
			go func() {
				for {
					msg := &packet.WorkerSessions{}
					if msg, err = client.Recv(); err != nil && err != io.EOF{
						log.PushLog("error receive session state from conductor %s", err.Error())
						done <- true
						break
					}
					ret.state_out <- msg
				}
			}()
			<-done
			cancel()
		}
	}()
	return ret, nil
}

func (ret *GRPCclient) genContext() context.Context {
	return metadata.NewOutgoingContext(
		context.Background(),
		metadata.Pairs(
			"username", ret.username,
			"password", ret.password,
		),
	)
}

func (client *GRPCclient) Stop() {
	client.conn.Close()
	client.connected = false
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
func (grpc *GRPCclient) Metric(log *packet.WorkerMetric) {
	grpc.monitoring <- log
}
func (grpc *GRPCclient) Infor(log *packet.WorkerInfor) {
	grpc.infor <- log
}
func (grpc *GRPCclient) Media(log *packet.MediaDevice) {
	grpc.devices <- log
}
func (grpc *GRPCclient) RecvSession() *packet.WorkerSessions {
	return <-grpc.state_out
}
func (grpc *GRPCclient) SyncSession(log *packet.WorkerSessions) {
	grpc.state_in <- log
}
