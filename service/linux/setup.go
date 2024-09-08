package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/service/cmd"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

func main() {
	exe, _ := os.Executable()
	dir, _ := filepath.Abs(filepath.Dir(exe))
	i := log.TakeLog(func(log string) {
		fmt.Println(log)
	})
	defer log.RemoveCallback(i)

	if log_file, err := os.OpenFile(fmt.Sprintf("%s/thinkmay.log", dir), os.O_RDWR|os.O_CREATE, 0755); err == nil {
		i := log.TakeLog(func(log string) {
			log_file.Write([]byte(fmt.Sprintf("%s\n", log)))
		})
		defer log.RemoveCallback(i)
	}

	chann := make(chan os.Signal, 16)
	go cmd.Start(chann)

	signal.Notify(chann, syscall.SIGTERM, os.Interrupt)
	chann <- <-chann

	log.PushLog("Stopped.")
	time.Sleep(3 * time.Second)
}
