package httpp

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling"
	ws "github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol/websocket"
)

func TestHttpServer(t *testing.T) {
	grpc, err := InitHttppServer()
	if err != nil {
		// log.PushLog("failed to setup grpc: %s", err.Error())
		fmt.Printf("Failed to setup grpc: %s", err.Error())
		return
	}
	defer grpc.Stop()

	signaling := signaling.InitSignallingServer(
		ws.InitSignallingHttp("/handshake/client"),
		ws.InitSignallingHttp("/handshake/server"),
	)

	srv := &http.Server{Addr: fmt.Sprintf(":%d", daemon.Httpport)}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.PushLog(err.Error())
		}
	}()
	defer srv.Close()

	log.PushLog("starting worker daemon")

	dm := daemon.WebDaemon(grpc, signaling, nil)
	defer dm.Close()

	// grpc.recv_session = func(*packet.WorkerSession, chan bool) (*packet.WorkerSession, error) {
	// 	grpc.Log("daemon", "infor", "hello")
	// 	return nil, nil
	// 	// fmt.Print("LOGGING %s", body)
	// }

	time.Sleep(8 * time.Minute)

}
