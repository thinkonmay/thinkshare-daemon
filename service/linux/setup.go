package main

import (
	"fmt"
	"io"
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
	if log_file, err := os.OpenFile(fmt.Sprintf("%s/thinkmay.log",dir), os.O_RDWR|os.O_CREATE, 0755); err == nil {
		i := log.TakeLog(func(log string) {
			str := fmt.Sprintf("daemon.exe : %s", log)
			log_file.Write([]byte(fmt.Sprintf("%s\n", str)))
			fmt.Println(str)
		})
		defer log.RemoveCallback(i)
		defer log_file.Close()
	}

	cluster := &daemon.ClusterConfig{}
	files, err := os.ReadFile(fmt.Sprintf("%s/cluster.yaml", dir))
	if err != nil {
		log.PushLog("failed to read cluster.yaml %s", err.Error())
		cluster = nil
	} else {
		app := pocketbase.New()
		app.Bootstrap()

		handle := func(c echo.Context) (err error) {
			body, _ := io.ReadAll(c.Request().Body)
			req, _ := http.NewRequest(
				c.Request().Method,
				fmt.Sprintf("http://localhost:60000%s?%s",
					c.Request().URL.Path,
					c.Request().URL.RawQuery),
				strings.NewReader(string(body)))

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.PushLog("error handle command %s : %s", c.Request().URL.Path, err.Error())
				return err
			}

			for k, v := range resp.Header {
				if len(v) == 0 {
					continue
				}
				c.Response().Header().Add(k, v[0])
			}

			body, _ = io.ReadAll(resp.Body)
			c.Response().Write(body)
			return nil
		}

		app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
			e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS(fmt.Sprintf("%s/web/dist", dir)), true))
			e.Router.GET("/info", handle)
			e.Router.POST("/new", handle)
			e.Router.POST("/closed", handle)
			e.Router.POST("/handshake/*", handle)
			return nil
		})

		go apis.Serve(app, apis.ServeConfig{
			ShowStartBanner:    true,
			HttpAddr:           "0.0.0.0:60080",
			HttpsAddr:          "0.0.0.0:60443",
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
