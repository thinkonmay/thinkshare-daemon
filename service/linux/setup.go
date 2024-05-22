package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/labstack/echo/v5"
	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/service/cmd"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"gopkg.in/yaml.v2"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
)

func main() {
	exe, _ := os.Executable()
	dir, _ := filepath.Abs(filepath.Dir(exe))
	i := log.TakeLog(func(log string) {
		str := fmt.Sprintf("daemon : %s", log)
		fmt.Println(str)
	})
	defer log.RemoveCallback(i)

	if log_file, err := os.OpenFile(fmt.Sprintf("%s/thinkmay.log", dir), os.O_RDWR|os.O_CREATE, 0755); err == nil {
		i := log.TakeLog(func(log string) {
			str := fmt.Sprintf("daemon : %s", log)
			log_file.Write([]byte(fmt.Sprintf("%s\n", str)))
		})
		defer log.RemoveCallback(i)
	}

	cluster := &daemon.ClusterConfig{}
	files, err := os.ReadFile(fmt.Sprintf("%s/cluster.yaml", dir))
	if err != nil {
		log.PushLog("failed to read cluster.yaml %s", err.Error())
		if ifaces, err := net.Interfaces(); err == nil {
			for _, local_if := range ifaces {
				if local_if.Flags&net.FlagLoopback > 0 ||
					local_if.Flags&net.FlagRunning == 0 {
					continue
				}

				cluster = &daemon.ClusterConfig{
					Nodes: []daemon.Node{},
					Local: daemon.Host{Interface: local_if.Name},
				}
				break
			}
		}
	} else {
		app := pocketbase.New()
		app.Bootstrap()

		client := http.Client{Timeout: 3 * time.Minute}
		handle := func(c echo.Context) (err error) {
			body, _ := io.ReadAll(c.Request().Body)
			req, _ := http.NewRequest(
				c.Request().Method,
				fmt.Sprintf("http://localhost:%d%s?%s",
					daemon.Httpport,
					c.Request().URL.Path,
					c.Request().URL.RawQuery),
				strings.NewReader(string(body)))

			resp, err := client.Do(req)
			if err != nil {
				log.PushLog("error handle command %s : %s", c.Request().URL.Path, err.Error())
				return err
			}

			body, err = io.ReadAll(resp.Body)
			if err != nil {
				return err
			} else if resp.StatusCode != 200 {
				c.Response().Status = resp.StatusCode
			}

			for k, v := range resp.Header {
				if len(v) == 0 || k == "Access-Control-Allow-Origin" || k == "Access-Control-Allow-Headers" {
					continue
				}
				c.Response().Header().Add(k, v[0])
			}

			c.Response().Write(body)
			return nil
		}

		app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
			e.Router.POST("/_new", handle)
			e.Router.POST("/new", handle)
			e.Router.POST("/closed", handle)
			e.Router.POST("/handshake/*", handle)
			e.Router.GET("/info", handle)
			e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS(fmt.Sprintf("%s/web/dist", dir)), true))
			return nil
		})

		go apis.Serve(app, apis.ServeConfig{
			ShowStartBanner:    true,
			HttpAddr:           "0.0.0.0:40080",
			HttpsAddr:          "0.0.0.0:40443",
			CertificateDomains: []string{"supabase.thinkmay.net"},
		})

		err = yaml.Unmarshal(files, cluster)
		if err != nil {
			log.PushLog("failed to read cluster.yaml %s", err.Error())
			cluster = nil
		}
	}

	chann := make(chan os.Signal, 16)
	go cmd.Start(cluster, chann)

	signal.Notify(chann, syscall.SIGTERM, os.Interrupt)
	chann <- <-chann

	log.PushLog("Stopped.")
	time.Sleep(3 * time.Second)
}