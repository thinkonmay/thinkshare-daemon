package media

import (
	"fmt"
	"testing"
	"time"
)

func TestDisplay(t *testing.T) {

	ActivateVirtualDriver()
	name,id := StartVirtualDisplay(1920,1080)
	fmt.Println(name)
	time.Sleep(1000 * time.Second)
	RemoveVirtualDisplay(id)
	time.Sleep(10 * time.Second)
	DeactivateVirtualDriver()
}