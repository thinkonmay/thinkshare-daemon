package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	httpp "github.com/thinkonmay/thinkshare-daemon/persistent/http"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol/websocket"
	"github.com/thinkonmay/thinkshare-daemon/utils/turn"
)

type StartRequest struct {
	daemon.DaemonOption
	Turn *struct {
		Username string `json:"username"`
		Password string `json:"password"`
		MinPort  int    `json:"min_port"`
		MaxPort  int    `json:"max_port"`
	}
}

func recv() *StartRequest {
	wait := make(chan *StartRequest)
	srv := &http.Server{Addr: ":60000"}
	defer func() { http.DefaultServeMux = http.NewServeMux() }()
	defer srv.Shutdown(context.Background())
	http.HandleFunc("/initialize", func(w http.ResponseWriter, r *http.Request) {
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
	go media.ActivateVirtualDriver()
	defer media.DeactivateVirtualDriver()

	if log_file, err := os.OpenFile("./thinkmay.log", os.O_RDWR|os.O_CREATE, 0755); err == nil {
		i := log.TakeLog(func(log string) {
			str := fmt.Sprintf("daemon.exe : %s", log)
			log_file.Write([]byte(fmt.Sprintf("%s\n", str)))
			fmt.Println(str)
		})
		defer log.RemoveCallback(i)
		defer log_file.Close()
	}

	req := recv()
	log.PushLog("received /initialize signal")
	if req.Turn != nil {
		turn.Open(req.Turn.Username, req.Turn.Password, req.Turn.MaxPort, req.Turn.MinPort)
		defer turn.Close()
	}

	grpc, err := httpp.InitHttppServer(req.Thinkmay.AccountID)
	if err != nil {
		log.PushLog("failed to setup grpc: %s", err.Error())
		return
	}
	defer grpc.Stop()

	signaling.InitSignallingServer(
		ws.InitSignallingWs("/handshake/client",func(r *http.Request) bool {return true}),
		ws.InitSignallingWs("/handshake/server",func(r *http.Request) bool {return r.Host == "localhost" || r.Host == "127.0.0.1"}),
	)

	srv := &http.Server{Addr: ":60000"}
	go srv.ListenAndServe()
	defer srv.Close()

	log.PushLog("starting worker daemon")
	dm := daemon.WebDaemon(grpc, daemon.DaemonOption{})
	defer dm.Close()
	<-stop
}
