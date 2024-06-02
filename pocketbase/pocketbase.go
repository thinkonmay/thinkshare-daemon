package pocketbase

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

func StartPocketbase(dir string, domain []string) {
	app := pocketbase.New()
	app.Bootstrap()

	client := http.Client{Timeout: 24 * time.Hour}
	handleauth := func(c echo.Context) (err error) {
		volumes := []struct {
			LocalID string `db:"local_id"`
		}{}

		err = app.App.Dao().ConcurrentDB().
			Select("volumes.local_id").
			From("volumes").
			AndWhere(dbx.Like("user", c.Request().Header.Get("User"))).
			All(&volumes)
		if err != nil {
			log.PushLog("error handle command %s : %s", c.Request().URL.Path, err.Error())
			return err
		}

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
		e.Router.GET("/_info", handle, apis.RequireAdminAuth())
		e.Router.GET("/info", handleauth)
		e.Router.GET("/*", apis.StaticDirectoryHandler(os.DirFS(fmt.Sprintf("%s/web/dist", dir)), true))
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
				log.PushLog("pocketbase error: %s",err.Error())
			}

			time.Sleep(time.Second)
		}
	}()
}
