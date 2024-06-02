package pocketbase

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAut(t *testing.T) {
	StartPocketbase("./test",[]string{})
	time.Sleep(time.Second * 5)
	req, _ := http.NewRequest("GET", "http://localhost:40080/info", strings.NewReader(""))
	req.Header.Add("User","vr2y6nl9859lenc")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%v\n",resp.Status)
	time.Sleep(time.Hour)
}
