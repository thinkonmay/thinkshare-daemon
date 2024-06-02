package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/pocketbase"
	"github.com/thinkonmay/thinkshare-daemon/service/cmd"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"gopkg.in/yaml.v2"
)

func main() {
	exe, _ := os.Executable()
	dir, _ := filepath.Abs(filepath.Dir(exe))
	i := log.TakeLog(func(log string) {
		str := fmt.Sprintf("daemon : %s", log)
		fmt.Println(str)
	})
	defer log.RemoveCallback(i)

	if log_file, err := os.OpenFile(fmt.Sprintf("%s/thinkmay.log", dir), os.O_RDWR|os.O_CREATE, 0755); err == nil {
		i := log.TakeLog(func(log string) {
			str := fmt.Sprintf("daemon : %s", log)
			log_file.Write([]byte(fmt.Sprintf("%s\n", str)))
		})
		defer log.RemoveCallback(i)
	}

	cluster := &daemon.ClusterConfig{}
	files, err := os.ReadFile(fmt.Sprintf("%s/cluster.yaml", dir))
	if err != nil {
		log.PushLog("failed to read cluster.yaml %s", err.Error())
		if ifaces, err := net.Interfaces(); err == nil {
			for _, local_if := range ifaces {
				if local_if.Flags&net.FlagLoopback > 0 ||
					local_if.Flags&net.FlagRunning == 0 {
					continue
				}

				cluster = &daemon.ClusterConfig{
					Nodes: []daemon.Node{},
					Local: daemon.Host{Interface: local_if.Name},
				}
				break
			}
		}
	} else {
		pocketbase.StartPocketbase("./pb_data", []string{"play.thinkmay.net"})
		err = yaml.Unmarshal(files, cluster)
		if err != nil {
			log.PushLog("failed to read cluster.yaml %s", err.Error())
			cluster = nil
		}
	}

	chann := make(chan os.Signal, 16)
	go cmd.Start(cluster, chann)

	signal.Notify(chann, syscall.SIGTERM, os.Interrupt)
	chann <- <-chann

	log.PushLog("Stopped.")
	time.Sleep(3 * time.Second)
}
