package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/persistent/websocket"
	"github.com/thinkonmay/thinkshare-daemon/utils/media"
	"github.com/thinkonmay/thinkshare-daemon/utils/turn"
)


func Start(stop chan bool) {
	media.ActivateVirtualDriver()
	defer media.DeactivateVirtualDriver()

	proxy_cred, err := credential.InputProxyAccount()
	if err != nil {
		fmt.Printf("failed to find proxy account: %s", err.Error())
		return
	}
	fmt.Println("proxy account found, continue")

	if ports,found := os.LookupEnv("BUILTIN_TURN"); found {
		portrange := strings.Split(ports, "-")
		if len(portrange) != 2 {
			fmt.Println("invalid port range")
		} 

		min,err := strconv.ParseInt(portrange[0], 10, 32)
		if err != nil {
			fmt.Println("invalid port range")
			min = 60000
		}
		max,err := strconv.ParseInt(portrange[1], 10, 32)
		if err != nil {
			fmt.Println("invalid port range")
			max = 65535
		}

		turn.Open(proxy_cred, int(min), int(max),)
		defer turn.Close()
    }

	worker_cred, err := credential.SetupWorkerAccount(proxy_cred)
	if err != nil {
		fmt.Printf("failed to setup worker account: %s", err.Error())
		return
	}

	grpc, err := websocket.InitGRPCClient(
		credential.PROJECT,
		credential.API_VERSION,
		credential.ANON_KEY,
		worker_cred)
	if err != nil {
		fmt.Printf("failed to setup grpc: %s", err.Error())
		return
	}

	defer grpc.Stop()
	dm := daemon.NewDaemon(grpc)
	defer dm.Close()
	<-stop
}
