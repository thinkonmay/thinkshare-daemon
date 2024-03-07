package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

func TestService(t *testing.T) {
	b, _ := json.Marshal(StartRequest{
		daemon.DaemonOption{
			Thinkmay: &struct {
				AccountID string "json:\"account_id\""
			}{
				AccountID: "hello",
			},
			Sunshine: nil,
		},
		nil,
	})

	resp, err := http.Post("http://localhost:60000/initialize", "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Error(err)
		return
	}

	b, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Error(fmt.Errorf(string(b)))
		return
	}

	fmt.Println(string(b))
}

func TestLog(t *testing.T) {
	resp, err := http.Get("http://localhost:60000/log")
	if err != nil {
		t.Error(err)
		return
	}

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Error(fmt.Errorf(string(b)))
		return
	}

	fmt.Println(string(b))
}

func TestInfo(t *testing.T) {
	resp, err := http.Get("http://localhost:60000/info")
	if err != nil {
		t.Error(err)
		return
	}

	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Error(fmt.Errorf(string(b)))
		return
	}

	fmt.Println(string(b))
}

func TestNew(t *testing.T) {
	b, _ := json.Marshal(packet.WorkerSession{
		Id:        0,
		Timestamp: "now",
		Sunshine:  nil,
		Thinkmay: &packet.ThinkmaySession{
			AuthConfig:   "",
			WebrtcConfig: "",
		},
	})

	resp, err := http.Post("http://localhost:60000/new", "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Error(err)
		return
	}

	b, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Error(fmt.Errorf(string(b)))
		return
	}

	fmt.Println(string(b))
}

func TestClose(t *testing.T) {
	b, _ := json.Marshal(struct {
		Id int `json:"id"`
	}{
		Id: 0,
	})

	resp, err := http.Post("http://localhost:60000/closed", "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Error(err)
		return
	}

	b, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Error(fmt.Errorf(string(b)))
		return
	}

	fmt.Println(string(b))
}
