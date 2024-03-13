package proxy

import (
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	go Server()
	time.Sleep(3 * time.Second)
	Client()
}
