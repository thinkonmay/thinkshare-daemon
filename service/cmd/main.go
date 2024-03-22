package cmd

import (
	"net/http"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	httpp "github.com/thinkonmay/thinkshare-daemon/persistent/http"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling"
	ws "github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol/websocket"
)

type StartRequest struct {
	daemon.DaemonOption
}

func Start(stop chan bool) {
	media.ActivateVirtualDriver()
	defer media.DeactivateVirtualDriver()

	grpc, err := httpp.InitHttppServer()
	if err != nil {
		log.PushLog("failed to setup grpc: %s", err.Error())
		return
	}
	defer grpc.Stop()

	signaling.InitSignallingServer(
		ws.InitSignallingHttp("/handshake/client", func(r *http.Request) bool { return true }),
		ws.InitSignallingHttp("/handshake/server", func(r *http.Request) bool { return true }),
	)

	srv := &http.Server{Addr: ":60000"}
	go srv.ListenAndServe()
	defer srv.Close()

	log.PushLog("starting worker daemon")
	dm := daemon.WebDaemon(grpc, daemon.DaemonOption{})
	defer dm.Close()
	<-stop
}