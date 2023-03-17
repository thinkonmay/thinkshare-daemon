package grpc

import (
	"context"
	"fmt"
	"time"

	"github.com/thinkonmay/conductor/protocol/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type GRPCclient struct {
	conn 		*grpc.ClientConn
	stream       packet.ConductorClient

	username string
	password string

	logger  	chan *packet.WorkerLog
	monitoring 	chan *packet.WorkerMetric
	infor   	chan *packet.WorkerInfor
	devices 	chan *packet.MediaDevice

	state_out   chan *packet.WorkerSession
	state_in    chan *packet.WorkerSession

	done      bool
	connected bool
}

func InitGRPCClient(host string, port int) (ret *GRPCclient, err error) {
	ret = &GRPCclient{
		connected: false,
		done:      false,
	}

	ret.conn, err = grpc.Dial(
		fmt.Sprintf("%s:%d", host, port),
		grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		return nil,err
	}

	ret.stream = packet.NewConductorClient(ret.conn)
	return ret,nil;
}

func (ret *GRPCclient)Initialize(username string, password string) {
	ret.username = username
	ret.password = password

	go func() {
		for {
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
					break;
				}
			}
		}
	}()
	go func() {
		for {
			client, err := ret.stream.Monitor(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				continue
			}

			for {
				if err := client.Send(<-ret.monitoring); err != nil {
					log.PushLog("error sending log to conductor %s", err.Error())
					break;
				}
			}
		}
	}()
	go func() {
		for {
			client, err := ret.stream.Mediadevice(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				continue
			}

			for {
				if err := client.Send(<-ret.devices); err != nil {
					log.PushLog("error sending log to conductor %s", err.Error())
					break;
				}
			}
		}
	}()	
	go func() {
		for {
			client, err := ret.stream.Infor(ret.genContext())
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				continue
			}

			for {
				if err := client.Send(<-ret.infor); err != nil {
					log.PushLog("error sending log to conductor %s", err.Error())
					break;
				}
			}
		}
	}()
	go func() {
		for {
			ctx,cancel := context.WithCancel(ret.genContext())
			client, err := ret.stream.Sync(ctx)
			if err != nil {
				fmt.Printf("fail to request stream: %s\n", err.Error())
				continue
			}

			done := make(chan bool, 2)
			go func() {
				for {
					if err := client.Send(<-ret.state_in); err != nil {
						log.PushLog("error sending log to conductor %s", err.Error())
						done<-true
						break;
					}
				}
			}()
			go func() {
				for {
					msg := &packet.WorkerSession{}
					if msg,err = client.Recv(); err != nil {
						log.PushLog("error sending log to conductor %s", err.Error())
						done<-true
						break;
					}
					ret.state_out<-msg
				}
			}()
			<-done
			cancel()
		}
	}()

	return
}



func (ret *GRPCclient)genContext() context.Context {
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
	client.done      = true 
}
