package cmd

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/cluster"
	httpp "github.com/thinkonmay/thinkshare-daemon/persistent/http"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"gopkg.in/yaml.v3"

	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/signaling"
	ws "github.com/thinkonmay/thinkshare-daemon/utils/signaling/protocol/websocket"
)

func Start(stop chan os.Signal) {
	media.ActivateVirtualDriver()
	defer media.DeactivateVirtualDriver()

	grpc, err := httpp.InitHttppServer()
	if err != nil {
		log.PushLog("failed to setup grpc: %s", err.Error())
		return
	}
	defer grpc.Stop()

	signaling := signaling.InitSignallingServer(
		ws.InitSignallingHttp(grpc.Mux,"/handshake/client"),
		ws.InitSignallingHttp(grpc.Mux,"/handshake/server"),
	)

	srv := &http.Server{
		Handler: grpc.Mux,
		Addr: fmt.Sprintf(":%d", daemon.Httpport), 
	}
	go func() {
		if pid, found := findPreviousPID(daemon.Httpport); found {
			log.PushLog("kill previous child process %s", pid)
			exec.Command("kill", pid).Run()
			time.Sleep(time.Second * 5)
		}

		for {
			if err := srv.ListenAndServe(); err != nil {
				log.PushLog(err.Error())
			}
			time.Sleep(time.Second)
		}
	}()
	defer srv.Close()

	log.PushLog("starting worker daemon")

	exe, _ := os.Executable()
	base_dir, _ := filepath.Abs(filepath.Dir(exe))
	manifest := fmt.Sprintf("%s/cluster.yaml", base_dir)
	if _, err := os.Stat(manifest); errors.Is(err, os.ErrNotExist) {
		i := (*net.Interface)(nil)
		if ifaces, err := net.Interfaces(); err == nil {
			for _, local_if := range ifaces {
				if local_if.Flags&net.FlagLoopback > 0 ||
					local_if.Flags&net.FlagRunning == 0 {
					continue
				}

				i = &local_if
				break
			}
		}

		if i == nil {
			panic(fmt.Errorf("no log file available"))
		}

		content, _ := yaml.Marshal(cluster.ClusterConfigManifest{
			Nodes: []cluster.NodeManifest{},
			Peers: []cluster.PeerManifest{},
			Local: cluster.Host{
				Interface: i.Name,
			},
		})

		os.WriteFile(manifest, content, 0777)
	}

	dm := daemon.WebDaemon(grpc, signaling, manifest)
	defer dm.Close()
	stop <- <-stop
}

func findPreviousPID(port int) (string, bool) {
	out, err := exec.Command("lsof", "-i", fmt.Sprintf(":%d", port)).Output()
	if err != nil {
		return "", false
	}

	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, "(LISTEN)") {
			continue
		}

		pos := 1
		for _, word := range strings.Split(line, " ") {
			if len(word) == 0 {
				continue
			}
			if pos == 2 {
				return word, true
			}
			pos++
		}
	}

	return "", false
}
