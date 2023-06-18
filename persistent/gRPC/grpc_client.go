package grpc

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	largerWindowSize = 65535 + 100 // https://github.com/grpc/grpc-go/issues/5358
)
type GRPCclient struct {
	stream packet.ConductorClient

	username string
	password string

	logger     chan *packet.WorkerLog
	monitoring chan *packet.WorkerMetric
	infor      chan *packet.WorkerInfor
	devices    chan *packet.MediaDevice
	storage    chan *packet.StorageStatus

	state_out chan *packet.WorkerSessions
	state_in  chan *packet.WorkerSessions

	chunk_in  chan *packet.StorageChunk
	chunk_out chan *packet.StorageChunk

	done      bool
	connected bool
}

func InitGRPCClient(host string,
					port int,
					account credential.Account,
					) (ret *GRPCclient, err error) {

	ret = &GRPCclient{
		connected: false,
		done:      false,

		username : *account.Username,
		password : *account.Password,

		logger     : make(chan *packet.WorkerLog,100),
		monitoring : make(chan *packet.WorkerMetric,100),
		infor      : make(chan *packet.WorkerInfor,100),
		devices    : make(chan *packet.MediaDevice,100),
		storage    : make(chan *packet.StorageStatus,100),

		state_out : make(chan *packet.WorkerSessions,100),
		state_in  : make(chan *packet.WorkerSessions,100),

		chunk_in  : make(chan *packet.StorageChunk,100),
		chunk_out : make(chan *packet.StorageChunk,100),
	}


	go func() {
		var conn *grpc.ClientConn = nil
		for {
			if ret.connected {
				time.Sleep(100 * time.Millisecond)
				continue
			} else if conn != nil {
				conn.Close()
			}

			conn, err = grpc.Dial(
				fmt.Sprintf("%s:%d", host, port),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithInitialWindowSize(largerWindowSize),
				grpc.WithInitialConnWindowSize(largerWindowSize),
			)

			if err != nil {
				fmt.Printf("failed to dial : %s",err.Error())
				time.Sleep(100 * time.Millisecond)
				continue
			}

			ret.stream = packet.NewConductorClient(conn)
			ret.connected = true
		}	
	}()


	go func() {
		for {
			if ret.done {
				return
			} else if !ret.connected {
				time.Sleep(100 * time.Millisecond)
				continue
			} 

			client, err := ret.stream.Storagesync(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				ret.connected = false
				continue
			}

			for {
				msg := <-ret.storage
				if err := client.Send(msg); err != nil && err != io.EOF{
					log.PushLog("error sending log to conductor %s", err.Error())
					ret.storage <- msg
					ret.connected = false
					break
				}
			}
		}
	}()
	go func() {
		for {
			if ret.done {
				return
			} else if !ret.connected {
				time.Sleep(100 * time.Millisecond)
				continue
			} 

			client, err := ret.stream.Logger(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				ret.connected = false
				continue
			}

			for {
				msg := <-ret.logger
				if err := client.Send(msg); err != nil && err != io.EOF{
					log.PushLog("error sending log to conductor %s", err.Error())
					ret.logger <- msg
					ret.connected = false
					break
				}
			}
		}
	}()
	go func() {
		for {
			if ret.done {
				return
			} else if !ret.connected {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			client, err := ret.stream.Monitor(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				ret.connected = false
				continue
			}

			for {
				msg := <-ret.monitoring
				if err := client.Send(msg); err != nil && err != io.EOF{
					log.PushLog("error sending metric to conductor %s", err.Error())
					ret.monitoring<-msg
					ret.connected = false
					break
				}
			}
		}
	}()
	go func() {
		for {
			if ret.done {
				return
			} else if !ret.connected {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			client, err := ret.stream.Mediadevice(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				ret.connected = false
				continue
			}

			for {
				dv := <-ret.devices
				if err := client.Send(dv); err != nil && err != io.EOF{
					log.PushLog("error sync media device : %s", err.Error())
					ret.connected = false
					break
				}
			}
		}
	}()
	go func() {
		for {
			if ret.done {
				return
			} else if !ret.connected {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			client, err := ret.stream.Infor(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				ret.connected = false
				continue
			}

			for {
				msg := <-ret.infor
				if err := client.Send(msg); err != nil && err != io.EOF{
					log.PushLog("error sending hwinfor to conductor %s", err.Error())
					ret.infor <- msg
					ret.connected = false
					break
				}
			}
		}
	}()
	go func() {
		for {
			if ret.done {
				return
			} else if !ret.connected {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			client, err := ret.stream.Sync(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				ret.connected = false
				continue
			}

			done := make(chan bool, 2)
			go func() {
				for {
					msg :=<- ret.state_in
					if err := client.Send(msg); err != nil && err != io.EOF{
						log.PushLog("error sending session state to conductor %s", err.Error())
						ret.state_in <- msg
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
			ret.connected = false
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

func (grpc *GRPCclient) StorageReport(status *packet.StorageStatus) {
	grpc.storage <- status
}
func (grpc *GRPCclient) StorageUpload() *packet.StorageChunk{
	return <-grpc.chunk_out
}
func (grpc *GRPCclient) StorageReceive(chunk *packet.StorageChunk) {
	grpc.chunk_in <- chunk
}