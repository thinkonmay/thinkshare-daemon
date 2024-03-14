package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	httpp "github.com/thinkonmay/thinkshare-daemon/persistent/http"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling"
	ws "github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol/websocket"
	"github.com/thinkonmay/thinkshare-daemon/utils/turn"
)

type StartRequest struct {
	daemon.DaemonOption
}

func recv() *StartRequest {
	wait := make(chan *StartRequest)
	srv := &http.Server{Addr: ":60000"}
	defer func() { http.DefaultServeMux = http.NewServeMux() }()
	defer srv.Shutdown(context.Background())
	http.HandleFunc("/initialize", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(503)
			w.Write([]byte(err.Error()))
			return
		}

		start := StartRequest{}
		err = json.Unmarshal(b, &start)
		if err != nil {
			w.WriteHeader(503)
			w.Write([]byte(err.Error()))
			return
		}

		if start.Turn != nil {
			port, err := credential.GetFreeUDPPort(start.Turn.MinPort, start.Turn.MaxPort)
			if err != nil {
				log.PushLog("failed to setup turn account: %s", err.Error())
				return
			}

			w.Write([]byte(fmt.Sprintf("{\"turn_port\": %d}", port)))
			start.Turn.Port = port
		} else {
			w.Write([]byte("{}"))
		}

		wait <- &start
	})

	go func() {
		for {
			err := srv.ListenAndServe()
			if err == http.ErrServerClosed {
				return
			}

			log.PushLog(err.Error())
			time.Sleep(time.Second)
		}
	}()

	log.PushLog("waiting for initialization at /initialize")
	return <-wait
}

func Start(stop chan bool) {
	media.ActivateVirtualDriver()
	defer media.DeactivateVirtualDriver()

	req := recv()
	log.PushLog("received /initialize signal")
	if req.Turn != nil {
		s, err := turn.Open(req.Turn.Username,
			req.Turn.Password,
			req.Turn.MaxPort,
			req.Turn.MinPort,
			req.Turn.Port)
		if err != nil {
			log.PushLog("failed start TURN server %s", err.Error())
		} else {
			defer s.Close()
		}
	}


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