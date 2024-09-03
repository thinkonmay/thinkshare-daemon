package pocketbase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

var (
	client = http.Client{Timeout: 24 * time.Hour}
	app    = (*pocketbase.PocketBase)(nil)
	doms   = []string{}
)

func StartPocketbase(dir, service_domain, admin_domain string) {
	doms = append(doms, service_domain, admin_domain)
	app := pocketbase.New()
	app.Bootstrap()

	path, _ := filepath.Abs(dir)
	log.PushLog("serving file content at %s", path)
	dirfs := os.DirFS(path)
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.POST("/_new", handle)
		e.Router.POST("/_use", handle)
		e.Router.POST("/new", handle)
		e.Router.POST("/closed", handle)
		e.Router.POST("/handshake/*", handle)
		e.Router.GET("/_info", handle, apis.RequireAdminAuth())
		e.Router.GET("/info", infoauth)

		// proxy API
		e.Router.Any("/auth/v1/callback", proxy("http://auth:9999", "/auth/v1/callback"))
		e.Router.Any("/auth/v1/authorize", proxy("http://auth:9999", "/auth/v1/authorize"))
		e.Router.Any("/auth/v1/verify", proxy("http://auth:9999", "/auth/v1/verify"))

		e.Router.Any("/auth/v1/*", proxy("http://auth:9999", "/auth/v1"))
		e.Router.Any("/rest/v1/*", proxy("http://rest:3000", "/rest/v1"))
		e.Router.Any("/pg/*", proxy("http://:8080", "/pg"))

		e.Router.GET("/*", func(c echo.Context) error {
			if c.Request().Host == service_domain {
				return apis.StaticDirectoryHandler(dirfs, true)(c)
			} else if c.Request().Host == admin_domain {
				return proxy("http://studio:3000", "")(c)
			} else {
				return c.Redirect(304, fmt.Sprintf("https://%s/"))
			}
		})
		return nil
	})

	go func() {
		for {
			_, err := apis.Serve(app, apis.ServeConfig{
				ShowStartBanner:    true,
				HttpAddr:           "0.0.0.0:80",
				HttpsAddr:          "0.0.0.0:443",
				CertificateDomains: doms,
			})
			if err != nil {
				log.PushLog("pocketbase error: %s", err.Error())
			}

			time.Sleep(time.Second)
		}
	}()
}

func infoauth(c echo.Context) (err error) {
	volumes := []struct {
		LocalID string `db:"local_id"`
	}{}

	user := c.Request().Header.Get("User")
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

func handle(c echo.Context) (err error) {
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

func proxy(destination, strip string) echo.HandlerFunc {
	return func(c echo.Context) error {
		curl, err := url.Parse(destination)
		if err != nil {
			return c.String(400, err.Error())
		}

		url := c.Request().URL
		url.Host = curl.Host
		url.Scheme = curl.Scheme

		new_path := " "
		if strip == "" {
			new_path = url.String()
		} else {
			new_path = strings.ReplaceAll(url.String(), strip, "")
		}

		req, err := http.NewRequest(
			c.Request().Method,
			new_path,
			c.Request().Body,
		)
		if err != nil {
			return c.String(400, err.Error())
		}

		req.Header = c.Request().Header.Clone()
		if resp, err := http.DefaultClient.Do(req); err != nil {
			return c.String(400, err.Error())
		} else {
			return c.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
		}
	}
}
