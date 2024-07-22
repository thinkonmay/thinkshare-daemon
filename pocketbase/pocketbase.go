package pocketbase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

const (
	Httpport = 60000
)

func StartPocketbase(dir string, domain []string) {
	app := pocketbase.New()
	app.Bootstrap()

	client := http.Client{Timeout: 24 * time.Hour}
	infoauth := func(c echo.Context) (err error) {
		volumes := []struct {
			LocalID string `db:"local_id"`
		}{}

		user := c.Request().Header.Get("User")
		log.PushLog("request from user %s", user)
		err = app.App.Dao().ConcurrentDB().
			Select("volumes.local_id").
			From("volumes").
			Where(dbx.NewExp("user = {:id}", dbx.Params{"id": user})).
			All(&volumes)
		if err != nil {
			log.PushLog("error handle command %s : %s", c.Request().URL.Path, err.Error())
			return err
		}

		vols := []string{}
		for _, v := range volumes {
			vols = append(vols, v.LocalID)
		}

		body, _ := io.ReadAll(c.Request().Body)
		req, _ := http.NewRequest(
			c.Request().Method,
			fmt.Sprintf("http://localhost:%d%s?%s",
				Httpport,
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

		data := packet.WorkerInfor{}
		json.Unmarshal(body, &data)

		newSessions := []*packet.WorkerSession{}
		for _, session := range data.Sessions {
			if session.Vm == nil || session.Vm.Volumes == nil {
				continue
			}

			found := false
			for _, volume := range session.Vm.Volumes {
				for _, vol := range vols {
					if vol == volume {
						found = true
					}
				}
			}

			if found {
				newSessions = append(newSessions, session)
			}
		}

		data.Sessions = newSessions

		newVolumes := []string{}
		for _, volume := range data.Volumes {
			found := false
			for _, vol := range vols {
				if vol == volume {
					found = true
				}
			}

			if found {
				newVolumes = append(newVolumes, volume)
			}
		}

		data.Volumes = newVolumes

		out, _ := json.Marshal(&data)
		c.Response().Write(out)
		return nil
	}

	handle := func(c echo.Context) (err error) {
		path := c.Request().URL.Path
		if path == "_info" {
			path = "info"
		}

		body, _ := io.ReadAll(c.Request().Body)
		req, _ := http.NewRequest(
			c.Request().Method,
			fmt.Sprintf("http://localhost:%d%s?%s",
				Httpport, path,
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

	// Customize edit manage volume api external
	handle_manage_volume := func(c echo.Context) (err error) {
		body, _ := io.ReadAll(c.Request().Body)
		req, _ := http.NewRequest(
			c.Request().Method,
			fmt.Sprintf("http://localhost:%d%s?%s",
				9000, c.Request().URL.Path,
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

	path, _ := filepath.Abs(fmt.Sprintf("%s/web/dist", dir))
	log.PushLog("serving file content at %s", path)
	dirfs := os.DirFS(path)
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.POST("/_new", handle)
		e.Router.POST("/new", handle)
		e.Router.POST("/closed", handle)
		e.Router.POST("/handshake/*", handle)
		e.Router.GET("/_info", handle, apis.RequireAdminAuth())
		e.Router.GET("/info", infoauth)

		// Customize edit manage volume api external
		e.Router.POST("/access_store_volume", handle_manage_volume, apis.RequireRecordAuth("users"))
		e.Router.POST("/check_volume", handle_manage_volume, apis.RequireAdminAuth())
		e.Router.POST("/volume_delete", handle_manage_volume, apis.RequireAdminAuth())
		e.Router.POST("/create_volume", handle_manage_volume, apis.RequireAdminAuth())
		e.Router.POST("/fetch_node_info", handle_manage_volume, apis.RequireAdminAuth())
		e.Router.POST("/fetch_node_volume", handle_manage_volume, apis.RequireAdminAuth())

		e.Router.GET("/*", apis.StaticDirectoryHandler(dirfs, true))
		return nil
	})

	go func() {
		for {
			_, err := apis.Serve(app, apis.ServeConfig{
				ShowStartBanner:    true,
				HttpAddr:           "0.0.0.0:40080",
				HttpsAddr:          "0.0.0.0:40443",
				CertificateDomains: domain,
			})
			if err != nil {
				log.PushLog("pocketbase error: %s", err.Error())
			}

			time.Sleep(time.Second)
		}
	}()
}
