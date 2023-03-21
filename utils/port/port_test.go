package port

import (
	"fmt"
	"testing"
)

func TestPort(t *testing.T) {
	port,err := GetFreePort()
	if err != nil {
		t.Error(err.Error())
	}

	fmt.Printf("port %d is open\n",port)

}