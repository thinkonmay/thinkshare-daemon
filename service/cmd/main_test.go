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

var (
	port = 60000
)

func TestService(t *testing.T) {
	b, _ := json.Marshal(StartRequest{
		daemon.DaemonOption{
			Turn: &struct {
				Username string "json:\"username\""
				Password string "json:\"password\""
				MinPort  int    "json:\"min_port\""
				MaxPort  int    "json:\"max_port\""
				Port int
			}{
				Username: "abc",
				Password: "bcd",
				MinPort:  60000,
				MaxPort:  65535,
			},
		},
	})

	resp, err := http.Post("http://192.168.1.11:60000/initialize", "application/json", strings.NewReader(string(b)))
	if err != nil {
		t.Error(err)
		return
	}

	b, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Error(fmt.Errorf(string(b)))
		return
	}

	data := map[string]int{}
	err = json.Unmarshal(b,&data)
	if err != nil {
		t.Error(err)
		return
	}

	port = data["turn_port"]
	fmt.Printf("%d\n",port)
}

func TestLog(t *testing.T) {
	resp, err := http.Get("http://192.168.1.11:60000/log")
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
	resp, err := http.Get("http://192.168.1.11:60000/info")
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
	str, _ := json.Marshal(map[string][]map[string]interface{}{
		"iceServers": 
		{ 
			{
				"urls": fmt.Sprintf("stun:192.168.1.11:%d", port),
			}, 
			{
				"urls": fmt.Sprintf("turn:192.168.1.11:%d", port),
				"username": "abc",
				"credential": "bcd",
			},
		},
	})
	b, _ := json.Marshal(packet.WorkerSession{
		Id:        0,
		Timestamp: "now",
		Sunshine:  nil,
		Thinkmay: &packet.ThinkmaySession{
			AuthConfig:   "",
			WebrtcConfig: string(str),
		},
	})

	resp, err := http.Post("http://192.168.1.11:60000/new", "application/json", strings.NewReader(string(b)))
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
	return
	b, _ := json.Marshal(struct {
		Id int `json:"id"`
	}{
		Id: 0,
	})

	resp, err := http.Post("http://192.168.1.11:60000/closed", "application/json", strings.NewReader(string(b)))
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
