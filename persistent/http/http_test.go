package httpp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

func TestT(t *testing.T) {
	client, err := InitHttppServer()
	if err != nil {
		panic(err)
	}

	go http.ListenAndServe(":60000", nil)

	client.RecvSession(func(ws *packet.WorkerSession, c chan bool) (*packet.WorkerSession, error) {
		fmt.Printf("%s request\n", time.Now().Format(time.RFC3339))
		<-c
		fmt.Printf("%s cancel\n", time.Now().Format(time.RFC3339))
		return nil, fmt.Errorf("cancel")
	})

	data, _ := json.Marshal(packet.WorkerSession{
		Id: uuid.NewString(),
	})

	go func() {
		for i := 0; i < 10; i++ {
			http.Post("http://localhost:60000/_new", "application/json", strings.NewReader(string(data)))
			time.Sleep(time.Second * 3)
		}

		fmt.Printf("%s stop ping \n", time.Now().Format(time.RFC3339))
	}()

	fmt.Printf("%s make request \n", time.Now().Format(time.RFC3339))
	http.Post("http://localhost:60000/new", "application/json", strings.NewReader(string(data)))
	fmt.Printf("%s done request \n", time.Now().Format(time.RFC3339))
	time.Sleep(time.Minute)
}
