package server

import (
	"crypto/md5"
	"fmt"
	"net"
	"os"
	"path"
	"time"

	"github.com/google/uuid"
	mlspb "github.com/thinkonmay/thinkshare-daemon/utils/filerpc/protobuf"
	"google.golang.org/grpc"
)

const (
	timeout_sec = 5
)

// MLSServer
type MLSServer struct {
	mlspb.UnimplementedMLSServiceServer
	documentDir string
}

// NewMLSServer
func NewMLSServer(documentDir string, port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		fmt.Printf("failed %s\n", err.Error())
		os.Exit(1)
	}
	defer lis.Close()

	fmt.Printf("Now serving %s\n", lis.Addr().String())

	grpcServer := grpc.NewServer()
	s := &MLSServer{documentDir: documentDir}
	mlspb.RegisterMLSServiceServer(grpcServer,s)
	if err = grpcServer.Serve(lis); err != nil {
		return err
	}

	return nil
}


func (ms *MLSServer) Upload(stream mlspb.MLSService_UploadServer) (err error) {
	md, err := expandMetaData(stream.Context())
	if err != nil {
		return fmt.Errorf("failed to retrieve incoming metadata: %s", err.Error())
	}

	size := md.fileSize
	tempFile := path.Join(ms.documentDir, fmt.Sprintf("%s.%s.temp", md.fileName, uuid.NewString()))
	destFile := path.Join(ms.documentDir, md.fileName)

	fmt.Printf("Begin receiving file %s\n", tempFile)
	f, err := os.OpenFile(tempFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %s", tempFile, err.Error())
	}

	channel := make(chan *mlspb.Chunk, 4096)
	now := func() int64 { return time.Now().UnixNano() }
	last_receive := now()
	last_id := 0

	go func() {
		for {
			data, err := stream.Recv()
			if err != nil {
				channel <- nil
				break
			}
			last_receive = now()
			channel <- data
		}
	}()

	go func() {
		for {
			time.Sleep(time.Millisecond * 100)
			if (now() - last_receive) < timeout_sec*time.Second.Nanoseconds() {
				channel <- nil
				return
			}
		}
	}()

	for {
		data := <-channel
		if data == nil {
			break
		}

		if last_id != int(data.Id)-1 {
			last_receive = now()
			channel <- data
			continue
		} else if data.Sum256 != fmt.Sprintf("%x", md5.Sum(data.Content)) {
			err = fmt.Errorf("invalid checksum")
			break
		}

		last_id = int(data.Id)
		if _, err = f.Write(data.Content); err != nil {
			err = fmt.Errorf("error Writing data: %s", err.Error())
			break
		}
	}
	if err != nil {
		f.Close()
		return
	}

	stat, err := f.Stat()
	if err != nil {
		return
	}

	f.Close()
	if stat.Size() < md.fileSize {
		err = fmt.Errorf("only received %dmb out of %dmb for file %s", stat.Size()/1024/1024, size/1024/1024, destFile)
	}

	if err != nil {
		os.Remove(tempFile)
	} else {
		os.Remove(destFile)
		os.Rename(tempFile, destFile)
	}

	return
}
