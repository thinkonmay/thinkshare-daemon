package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	httpp "github.com/thinkonmay/thinkshare-daemon/persistent/http"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling"
	ws "github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol/websocket"
	"gopkg.in/yaml.v3"
)


func Start(stop chan bool) {
	media.ActivateVirtualDriver()
	defer media.DeactivateVirtualDriver()

	grpc, err := httpp.InitHttppServer()
	if err != nil {
		log.PushLog("failed to setup grpc: %s", err.Error())
		return
	}
	defer grpc.Stop()

	signaling := signaling.InitSignallingServer(
		ws.InitSignallingHttp("/handshake/client"),
		ws.InitSignallingHttp("/handshake/server"),
	)

	srv := &http.Server{Addr: ":60000"}
	go srv.ListenAndServe()
	defer srv.Close()

	log.PushLog("starting worker daemon")

	exe, _ := os.Executable()
	dir, _ := filepath.Abs(filepath.Dir(exe))
	cluster := &daemon.ClusterConfig{}
	files,err := os.ReadFile(fmt.Sprintf("%s/cluster.yaml",dir))
	if err != nil {
		log.PushLog("failed to read cluster.yaml %s",err.Error())
		cluster = nil
	} else {
		err = yaml.Unmarshal(files,cluster)
		if err != nil {
			log.PushLog("failed to read cluster.yaml %s",err.Error())
			cluster = nil
		}
	}


	dm := daemon.WebDaemon(grpc,signaling,cluster)
	defer dm.Close()
	<-stop
}
