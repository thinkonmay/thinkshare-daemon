package client

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	mlspb "github.com/thinkonmay/thinkshare-daemon/utils/filerpc/protobuf"
)

const (
	unittime = int64((512 * 1024) * (1000 * 1000 * 1000) / (50 * 1024 * 1024))
)

type MLSClient struct {
	conn  *grpc.ClientConn
	file  *os.File
	route mlspb.MLSServiceClient
}

func NewClient(file *os.File, addr string) error {

	// Set up connection with rpc server
	var conn *grpc.ClientConn
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(fmt.Errorf("grpc Dial fail: %s/n", err))
	}

	defer conn.Close()
	ret := &MLSClient{
		conn:  conn,
		file:  file,
		route: mlspb.NewMLSServiceClient(conn),
	}

	return ret.upload()
}

func (mc *MLSClient) upload() (err error) {
	ctx := context.Background()

	info, err := mc.file.Stat()
	if err != nil {
		return 
	}
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"file_name", info.Name(),
		"file_size", strconv.FormatInt(info.Size(), 10),
	))

	stream, err := mc.route.Upload(ctx)
	if err != nil {
		return 
	}

	channel := make(chan []byte, 1000)
	now := func() int64 { return time.Now().UnixNano() }

	go func() {
		n := 0
		prev := now()
		for {
			if next := now(); next-prev < unittime {
				time.Sleep(time.Duration(unittime - (next - prev)))
			}

			prev = now()
			buf := make([]byte, 1024*512) // 512KB chunk
			n, err = mc.file.Read(buf)
			if err != nil {
				channel <- nil
				return
			}

			channel <- buf[:n]
		}
	}()

	var count int64 = 1
	for {
		buf := <-channel
		if buf == nil {
			break
		}

		chunk := &mlspb.Chunk{
			Id:      count,
			Content: buf,
			Sum256:  fmt.Sprintf("%x", md5.Sum(buf)),
		}
		if err = stream.Send(chunk); err != nil {
			break
		}

		count++
	}

	if err == io.EOF {
		err = nil
	}
	return err
}
