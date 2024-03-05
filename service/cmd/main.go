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
	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/persistent"
	httpp "github.com/thinkonmay/thinkshare-daemon/persistent/http"
	"github.com/thinkonmay/thinkshare-daemon/persistent/websocket"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
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
	defer srv.Shutdown(context.Background())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}

		start := StartRequest{}
		err = json.Unmarshal(b, &start)
		if err != nil {
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

	return <-wait
}

func Start(stop chan bool) {
	media.ActivateVirtualDriver()
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
	if req.Turn != nil {
		turn.Open(req.Turn.Username, req.Turn.Password, req.Turn.MaxPort, req.Turn.MinPort)
		defer turn.Close()
	}

	grpc, err := func() (p persistent.Persistent, err error) {
		if req.Thinkmay != nil {
			p, err = websocket.InitWebsocketClient(
				credential.PROJECT,
				credential.API_VERSION,
				credential.ANON_KEY,
				credential.Account{&req.Thinkmay.Username, &req.Thinkmay.Password})
		} else if req.Sunshine != nil {
			p, err = httpp.InitHttppServer(
				credential.PROJECT,
				credential.API_VERSION,
				credential.ANON_KEY,
				credential.Account{&req.Sunshine.Username, &req.Thinkmay.Password})
		} else {
			err = fmt.Errorf("no available configuration")
		}

		return
	}()
	if err != nil {
		log.PushLog("failed to setup grpc: %s", err.Error())
		return
	}
	defer grpc.Stop()

	log.PushLog("starting worker daemon")
	dm := daemon.WebDaemon(grpc, daemon.DaemonOption{})
	defer dm.Close()
	<-stop
}
