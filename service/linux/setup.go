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
	if log_file, err := os.OpenFile("./thinkmay.log", os.O_RDWR|os.O_CREATE, 0755); err == nil {
		i := log.TakeLog(func(log string) {
			str := fmt.Sprintf("daemon.exe : %s", log)
			log_file.Write([]byte(fmt.Sprintf("%s\n", str)))
			fmt.Println(str)
		})
		defer log.RemoveCallback(i)
		defer log_file.Close()
	}

	end := make(chan bool)
	go cmd.Start(end)

	chann := make(chan os.Signal, 16)
	signal.Notify(chann, syscall.SIGTERM, os.Interrupt)
	<-chann
	end <- true

	log.PushLog("Stopped.")
	time.Sleep(3 * time.Second)
}