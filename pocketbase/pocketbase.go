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
	"github.com/thinkonmay/pocketbase"
	"github.com/thinkonmay/pocketbase/apis"
	"github.com/thinkonmay/pocketbase/core"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

const (
	Httpport = 60000
)

var (
	client = http.Client{Timeout: 24 * time.Hour}
	app    = (*pocketbase.PocketBase)(nil)
	doms   = struct{ ServiceDomain, MonitorDomain, AdminDomain, DataDomain string }{}
)

func StartPocketbase() {
	enable_https := false

	ok := false
	dir := "/web"
	certdoms := []string{}
	if doms.ServiceDomain, ok = os.LookupEnv("SERVICE_DOMAIN"); ok {
		certdoms = append(certdoms, doms.ServiceDomain)
	}
	if doms.MonitorDomain, ok = os.LookupEnv("MONITOR_DOMAIN"); ok {
		certdoms = append(certdoms, doms.MonitorDomain)
	}
	if doms.AdminDomain, ok = os.LookupEnv("ADMIN_DOMAIN"); ok {
		certdoms = append(certdoms, doms.AdminDomain)
	}
	if doms.DataDomain, ok = os.LookupEnv("DATA_DOMAIN"); ok {
		certdoms = append(certdoms, doms.DataDomain)
	}
	if enableSSL, ok := os.LookupEnv("ENABLE_HTTPS"); ok && enableSSL == "true" {
		enable_https = true
	}
	if _dir, ok := os.LookupEnv("WEB_DIR"); ok {
		dir = _dir
	}

	app := pocketbase.New()
	app.Bootstrap()

	path, _ := filepath.Abs(dir)
	dirfs := os.DirFS(path)

	pre := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			switch c.Request().Host {
			case doms.DataDomain:
				return proxy("http://studio:3000", "")(c)
			case doms.MonitorDomain:
				return proxy("http://grafana:3000", "")(c)
			default:
				return next(c)
			}
		}
	}
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
		e.Router.Any("/pg/*", proxy("http://meta:8080", "/pg"))

		e.Router.Any("/*", apis.StaticDirectoryHandler(dirfs, true))
		return nil
	})

	go func() {
		for {
			err := (error)(nil)
			config := apis.ServeConfig{
				ShowStartBanner: true,
				HttpAddr:        "0.0.0.0:80",
				PreMiddleware:   pre,
			}
			if enable_https {
				config.HttpsAddr = "0.0.0.0:443"
				config.CertificateDomains = certdoms
			}

			_, err = apis.Serve(app, config)
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
		req.Host = c.Request().Host
		if resp, err := http.DefaultClient.Do(req); err != nil {
			return c.String(400, err.Error())
		} else {
			for k, v := range resp.Header {
				if len(v) > 0 {
					c.Response().Header().Add(k, v[0])
				}
			}
			return c.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
		}
	}
}
