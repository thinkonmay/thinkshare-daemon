package pocketbase

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/thinkonmay/pocketbase"
	"github.com/thinkonmay/pocketbase/apis"
	"github.com/thinkonmay/pocketbase/core"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	ws "golang.org/x/net/websocket"
)

const (
	Httpport = 60000
)

var (
	client = http.Client{Timeout: 24 * time.Hour}
	app    = (*pocketbase.PocketBase)(nil)
	env   = struct{ ServiceDomain, MonitorDomain, AdminUsername, AdminPassword, AdminDomain, DataDomain, ManageVolumeDomain string }{}
)

func StartPocketbase() {
	ok := false
	dir := "./web"
	enable_https := true
	certdoms := []string{}
	if env.ServiceDomain, ok = os.LookupEnv("SERVICE_DOMAIN"); ok {
		certdoms = append(certdoms, env.ServiceDomain)
	} else {
		env.ServiceDomain = "play.thinkmay.net"
	}
	if env.MonitorDomain, ok = os.LookupEnv("MONITOR_DOMAIN"); ok {
		certdoms = append(certdoms, env.MonitorDomain)
	}
	if env.AdminDomain, ok = os.LookupEnv("ADMIN_DOMAIN"); ok {
		certdoms = append(certdoms, env.AdminDomain)
	}
	if env.AdminPassword, ok = os.LookupEnv("ADMIN_PASSWORD"); ok {
	}
	if env.AdminUsername, ok = os.LookupEnv("ADMIN_USERNAME"); ok {
	}
	if env.DataDomain, ok = os.LookupEnv("DATA_DOMAIN"); ok {
		certdoms = append(certdoms, env.DataDomain)
	}
	if env.ManageVolumeDomain, ok = os.LookupEnv("MANAGE_VOLUME"); ok {
		certdoms = append(certdoms, env.ManageVolumeDomain)
	}
	if enableSSL, ok := os.LookupEnv("DISABLE_HTTPS"); ok && enableSSL == "true" {
		enable_https = false
	}
	if _dir, ok := os.LookupEnv("WEB_DIR"); ok {
		dir = _dir
	}

	app = pocketbase.New()
	app.Bootstrap()

	path, _ := filepath.Abs(dir)
	dirfs := os.DirFS(path)

	expectedUsernameHash := sha256.Sum256([]byte(env.AdminUsername))
	expectedPasswordHash := sha256.Sum256([]byte(env.AdminPassword))
	basicAuth := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if username, password, ok := c.Request().BasicAuth(); ok {
				usernameHash := sha256.Sum256([]byte(username))
				passwordHash := sha256.Sum256([]byte(password))
				usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], expectedUsernameHash[:]) == 1)
				passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], expectedPasswordHash[:]) == 1)
				if usernameMatch && passwordMatch {
					return next(c)
				} else {
					c.Response().Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
					return c.String(http.StatusUnauthorized, "Unauthorized")
				}
			} else {
				c.Response().Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
				return c.String(http.StatusUnauthorized, "Unauthorized")
			}
		}
	}

	pre := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			switch c.Request().Host {
			case env.DataDomain:
				return basicAuth(proxy("http://studio:3000", "", ""))(c)
			case env.AdminDomain:
				return basicAuth(proxy("http://admin:3000", "", ""))(c)
			case env.MonitorDomain:
				return proxy("http://grafana:3000", "", "")(c)
			case env.ManageVolumeDomain:
				return proxy("http://manage_volume", "", "")(c) 
			case env.ServiceDomain:
				if c.IsWebSocket() {
					return proxy("http://realtime-dev.supabase-realtime:4000", "/realtime/v1", "/socket")(c)
				} else {
					return next(c)
				}
			default:
				return next(c)
			}
		}
	}
	recover := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if err := recover(); err != nil {
					log.PushLog("receive panic in serve thread: %s", debug.Stack())
				}
			}()
			return next(c)
		}
	}
	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.POST("/_new", handle, recover)
		e.Router.POST("/_use", handle, recover)
		e.Router.POST("/new", handle, recover)
		e.Router.POST("/closed", handle, recover)
		e.Router.POST("/handshake/*", handle, recover)
		e.Router.GET("/info", infoauth, recover)

		// proxy API
		e.Router.Any("/auth/v1/callback", proxy("http://auth:9999", "/auth/v1/callback", ""), recover)
		e.Router.Any("/auth/v1/authorize", proxy("http://auth:9999", "/auth/v1/authorize", ""), recover)
		e.Router.Any("/auth/v1/verify", proxy("http://auth:9999", "/auth/v1/verify", ""), recover)

		e.Router.Any("/auth/v1/*", proxy("http://auth:9999", "/auth/v1", ""), recover)
		e.Router.Any("/rest/v1/*", proxy("http://rest:3000", "/rest/v1", ""), recover)
		e.Router.Any("/realtime/v1/api/*", proxy("http://realtime-dev.supabase-realtime:4000", "/realtime/v1/api", "/api"), recover)
		e.Router.Any("/pg/*", proxy("http://meta:8080", "/pg", ""), recover)

		// volume API

		e.Router.Any("/access_store_volume", proxy("http://manage_volume", "/access_store_volume", "/access_store_volume"), recover)
		e.Router.Any("/create_volume", proxy("http://manage_volume", "/create_volume", "/create_volume"), recover)
		e.Router.Any("/volume_delete", proxy("http://manage_volume", "/volume_delete", "/volume_delete"), recover)
		e.Router.Any("/fetch_node_info", proxy("http://manage_volume", "/fetch_node_info", "/fetch_node_info"), recover)
		e.Router.Any("/fetch_node_volume", proxy("http://manage_volume", "/fetch_node_volume", "/fetch_node_volume"), recover)
		e.Router.Any("/check_volume", proxy("http://manage_volume", "/check_volume", "/check_volume"), recover)

		e.Router.Any("/*", apis.StaticDirectoryHandler(dirfs, true), recover)
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
		if len(v) == 0 ||
			k == "Access-Control-Allow-Origin" ||
			k == "Access-Control-Allow-Headers" {
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
		if len(v) == 0 ||
			k == "Access-Control-Allow-Origin" ||
			k == "Access-Control-Allow-Headers" {
			continue
		}
		c.Response().Header().Add(k, v[0])
	}

	c.Response().Write(body)
	return nil
}

func proxy(destination, strip, replace string) echo.HandlerFunc {
	get_path := func(c *http.Request, transform func(url *url.URL)) (string, error) {
		curl, err := url.Parse(destination)
		if err != nil {
			return "", err
		}

		url := c.URL
		url.Host = curl.Host
		url.Scheme = curl.Scheme
		transform(url)
		new_path := " "
		if strip == "" {
			new_path = url.String()
		} else {
			new_path = strings.ReplaceAll(url.String(), strip, replace)
		}
		return new_path, nil
	}

	handle_ws := func(c echo.Context) error {
		ws.Handler(func(ctx *ws.Conn) {
			path, connErr := get_path(c.Request(), func(url *url.URL) {
				url.Scheme = "ws"
			})
			if connErr != nil {
				return
			}

			header := c.Request().Header.Clone()
			delete(header, "Sec-Websocket-Extensions")
			delete(header, "Sec-Websocket-Version")
			delete(header, "Sec-Websocket-Key")
			delete(header, "Connection")
			delete(header, "Upgrade")
			conn, _, connErr := websocket.DefaultDialer.Dial(path, header)
			if connErr != nil {
				return
			}
			defer conn.Close()

			exitErr := (error)(nil)

			go func() {
				buffer := make([]byte, 4096)
				for {
					size, err := ctx.Read(buffer)
					if err != nil {
						exitErr = err
						break
					}

					if err := conn.WriteMessage(websocket.BinaryMessage, buffer[:size]); err != nil {
						exitErr = err
						break
					}
				}
			}()

			go func() {
				for {
					_, message, err := conn.ReadMessage()
					if err != nil {
						exitErr = err
						break
					}

					if _, err := ctx.Write(message); err != nil {
						exitErr = err
						break
					}
				}
			}()

			for exitErr == nil {
				time.Sleep(time.Millisecond * 100)
			}

		}).ServeHTTP(c.Response(), c.Request())
		return nil
	}

	return func(c echo.Context) error {
		new_path, err := get_path(c.Request(), func(url *url.URL) {})
		if err != nil {
			return err
		}

		if c.IsWebSocket() {
			return handle_ws(c)
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
				if len(v) == 0 ||
					k == "Access-Control-Allow-Origin" ||
					k == "Access-Control-Allow-Headers" {
					continue
				}

				c.Response().Header().Add(k, v[0])
			}
			return c.Stream(resp.StatusCode, resp.Header.Get("Content-Type"), resp.Body)
		}
	}
}
