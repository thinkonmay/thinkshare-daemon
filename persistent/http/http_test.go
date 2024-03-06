package httpp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

func TestHttp(t *testing.T) {
	server,err := InitHttppServer("hi")
	if err != nil {
		t.Error(err)
	}

	server.Log("hi","hi","hi")
	server.RecvSession(func(ws *packet.WorkerSession) error {
		fmt.Printf("%v\n",ws)
		return nil
	})
	go func() { 
		info,err := system.GetInfor()
		if err != nil {
			t.Error(err)
		}
		server.Infor(info)
	}()

	
	buf,_ := json.Marshal(packet.WorkerSession{
		Thinkmay: &packet.ThinkmaySession{
			AuthConfig: "hi",
		},
	})
	resp,err := http.DefaultClient.Post("http://localhost:60000/new",
		"application/json",
		strings.NewReader(string(buf)))
	if err != nil {
		t.Error(err)
	}

	fmt.Println(resp.Status)

	resp,err = http.DefaultClient.Get("http://localhost:60000/log")
	if err != nil {
		t.Error(err)
	}

	buf = make([]byte, 1024)
	n,_ := resp.Body.Read(buf)
	fmt.Println(string(buf[:n]))

	resp,err = http.DefaultClient.Get("http://localhost:60000/info")
	if err != nil {
		t.Error(err)
	}

	buf = make([]byte, 1024)
	n,_ = resp.Body.Read(buf)
	fmt.Println(string(buf[:n]))

	resp,err = http.DefaultClient.Post("http://localhost:60000/closed",
		"application/json",
		strings.NewReader(string("{ \"id\" : 0 }")))
	if err != nil {
		t.Error(err)
	}

	fmt.Println(resp.Status)

	server.ClosedSession()
}