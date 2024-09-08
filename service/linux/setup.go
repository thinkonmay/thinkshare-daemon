package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/service/cmd"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

func main() {
	i := log.TakeLog(func(log string) {
		fmt.Println(log)
	})
	defer log.RemoveCallback(i)

	chann := make(chan os.Signal, 16)
	go cmd.Start(chann)

	signal.Notify(chann, syscall.SIGTERM, os.Interrupt)
	chann <- <-chann

	log.PushLog("Stopped.")
	time.Sleep(3 * time.Second)
}
