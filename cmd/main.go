package main

import (
	"fmt"
	"os"
	"strconv"

	daemon "github.com/thinkonmay/thinkshare-daemon"
	"github.com/thinkonmay/thinkshare-daemon/credential"
	grpc "github.com/thinkonmay/thinkshare-daemon/persistent/gRPC"
)


type RunType int
const (
	failed = -1
	short_task = 0
	worker_node = 1
)


func main() {
	task,err := ShortTask()
	switch int(task) {
	case failed:
		fmt.Printf("failed : %s\n",err.Error())
		return
	case short_task:
		return
	case worker_node:
	}


	proxy_cred, err := credential.UseProxyAccount()
	if err != nil {
		fmt.Printf("failed to find proxy account: %s", err.Error())
		return
	}


	fmt.Println("proxy account found, continue")
	worker_cred, err := credential.SetupWorkerAccount(proxy_cred)
	if err != nil {
		fmt.Printf("failed to setup worker account: %s", err.Error())
		return
	}


	grpc,err := grpc.InitGRPCClient(
		credential.Secrets.Conductor.Hostname,
		credential.Secrets.Conductor.GrpcPort,
		worker_cred)
	if err != nil {
		fmt.Printf("failed to setup grpc: %s", err.Error())
		return
	}

	dm := daemon.NewDaemon(grpc)
	dm.TerminateAtTheEnd()
	<-dm.Shutdown
}





func ShortTask() (task RunType,err error) {
	out := ""
	task = worker_node
	for i,arg := range os.Args[1:]{ switch arg {
	case "proxy" :
	task = short_task
	for _,arg1 := range os.Args[i:]{ switch arg1 {
		case "current" :
		case "reset" :
			os.Remove(credential.ProxySecretFile)
		}
	}
	proxy_account,err := credential.SetupProxyAccount()
	if err != nil { return -1,err }
	out = fmt.Sprintf("proxy account generated : %s",proxy_account.Username)

	case "vendor" :
	task = short_task
	api_key,err := credential.SetupApiKey()
	if err != nil { return -1,err }
	for t,arg1 := range os.Args[i:]{ switch arg1 {
		case "keygen" :
			out = fmt.Sprintf("api key : %s",api_key.Key)
		case  "list-workers" :
			out,err = credential.FetchWorker(api_key)
			if err != nil { return -1,err }
		case  "create-session" :
			id,soundcard,monitor := -1,"Default Audio Render Device","Generic PnP Monitor"
			for v,arg2 := range os.Args[t:]{
				switch arg2{
				case  "--worker-id" :
					if id,err = strconv.Atoi(os.Args[t:][v+1]);err != nil { panic(err) }
				case  "--monitor" :
					monitor = os.Args[t:][v+1]
				case  "--soundcard" :
					soundcard = os.Args[t:][v+1]
				}
			}

			out,err = credential.CreateSession(credential.Filter{
				WorkerId: id,
				SoudcardName: soundcard,
				MonitorName: monitor},api_key)

			if err != nil { return -1,err }
		case  "deactivate-session" :
			id := -1
			for v,arg2 := range os.Args[t:]{
				switch arg2 {
				case  "--session-id" :
					if id,err = strconv.Atoi(os.Args[t:][v+1]);err != nil { panic(err) }
				}
			}

			out,err = credential.DeactivateSession(id,api_key)
			if err != nil { return -1,err }
		}
	}
	case "--help":
		printHelp()
	}
	}


	if task == short_task {
		fmt.Println(out)
		secret_f, err := os.OpenFile("./task_result.yaml", os.O_RDWR|os.O_CREATE, 0755)
		if err != nil { panic(err) }
		defer secret_f.Close()

		secret_f.Truncate(0)
		secret_f.WriteAt([]byte(out),0)
		return short_task,nil
	}

	return worker_node,nil
}



func printHelp() {
	fmt.Println("required environment (always): 		")
	fmt.Println(" - PROJECT: project id (ex: \"avmvymkexjarplbxwlnj\")		")
	fmt.Println("")
	fmt.Println("A. CLI to do simple tasks, format: ./daemon.exe proxy current")
	fmt.Println(" -proxy			")
	fmt.Println("    -current              (when) get current proxy account username (generate if none)	")
	fmt.Println("    -reset                (when) get new proxy account	")
	fmt.Println(" -vendor		")
	fmt.Println("    -keygen               (when) generate api key for vendor	")
	fmt.Println("    -list-workers         (when) list all workers belong to account	")
	fmt.Println("    -create-session       (when) create new worker session	")
	fmt.Println("        --worker-id       (use) daemon vendor create-session --worker-id 12	")
	fmt.Println("        --monior          (use) daemon vendor create-session --monitor \"\\\\.\\DISPLAY1\"	")
	fmt.Println("        --soundcard       (use) daemon vendor create-session --soundcard \"Default Audio Render Device\"     ")
	fmt.Println("    -deactivate-session   (when) deactivate running worker session	")
	fmt.Println("        --session-id      (use) daemon vendor deactivate-session --session-id 12	")
	fmt.Println("")
	fmt.Println("B. Run the worker node   ")
	fmt.Println("     1. ./daemon.exe proxy current  ")
	fmt.Println("     2. run ./scripts/installService.bat as Administrator ")
}