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
	switch int(ShortTask()) {
	case failed:
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





func ShortTask() (RunType) {
	var task RunType

	var err error
	out,worker_account,proxy_account,api_key := "",credential.Account{},credential.Account{},credential.ApiKey{}

	for i,arg := range os.Args[1:]{ switch arg {
	case "proxy" :
	task = short_task
	for _,arg1 := range os.Args[i:]{ switch arg1 {
		case "current" :
			proxy_account,err = credential.SetupProxyAccount()
			out = fmt.Sprintf("proxy account : %s",proxy_account.Username)
		case "reset" :
			os.Remove(credential.ProxySecretFile)
			proxy_account,err = credential.SetupProxyAccount()
			out = fmt.Sprintf("proxy account generated : %s",proxy_account.Username)
		}
	}
	case "vendor" :
	task = short_task
	for t,arg1 := range os.Args[i:]{ switch arg1 {
		case "keygen" :
			api_key,err = credential.SetupApiKey()
			out = fmt.Sprintf("api key : %s",api_key.Key)
		case  "list-workers" :
			api_key,err = credential.SetupApiKey()
			if err != nil {
				break
			}

			out,err = credential.FetchWorker(api_key)
		case  "create-session" :
			id,soundcard,monitor := -1,"Default Audio Render Device","Generic PnP Monitor"
			api_key,err = credential.SetupApiKey()
			if err != nil {
				break
			}
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
				MonitorName: monitor,
			},api_key)
		case  "deactivate-session" :
			id := -1
			if api_key,err = credential.SetupApiKey();err != nil { break }
			for v,arg2 := range os.Args[t:]{
				switch arg2 {
				case  "--session-id" :
					if id,err = strconv.Atoi(os.Args[t:][v+1]);err != nil { panic(err) }
				}
			}

			out,err = credential.DeactivateSession(id,api_key)
		}
	}
	case "--help":
		printHelp()
	}
	}


	

	if err != nil {
		fmt.Printf("failed : %s",err.Error())
		fmt.Sprintf("",worker_account,proxy_account,api_key)
		return failed
	} 
	
	if task == short_task {
		fmt.Println(out)
		secret_f, err := os.OpenFile("./task_result.yaml", os.O_RDWR|os.O_CREATE, 0755)
		if err != nil { panic(err) }
		defer secret_f.Close()

		secret_f.Truncate(0)
		secret_f.WriteAt([]byte(out),0)
		return short_task
	}

	return worker_node
}



func printHelp() {
	fmt.Println("required environment (always): 																				")
	fmt.Println(" - PROJECT: project id (ex: \"avmvymkexjarplbxwlnj\")														")
	fmt.Println("																												")
	fmt.Println("for short running task, you can check at ./task_result.yaml after finish										")
	fmt.Println("format :    ./daemon.exe proxy current																			")
	fmt.Println(" -proxy																										")
	fmt.Println("    -current              (when) get current proxy account username (generate if none)							")
	fmt.Println("    -reset                (when) get new proxy account															")
	fmt.Println("																												")
	fmt.Println(" -vendor																										")
	fmt.Println("    -keygen               (when) generate api key for vendor													")
	fmt.Println("    -list-workers         (when) list all workers belong to account											")
	fmt.Println("    -create-session       (when) create new worker session													    ")
	fmt.Println("        --worker-id       (use) daemon vendor create-session --worker-id 12									")
	fmt.Println("        --monior          (use) daemon vendor create-session --monitor \"\\\\.\\DISPLAY1\"						")
	fmt.Println("        --soundcard       (use) daemon vendor create-session --soundcard \"Default Audio Render Device\"		")
	fmt.Println("    -deactivate-session   (when) deactivate running worker session												")
	fmt.Println("        --session-id      (use) daemon vendor deactivate-session --session-id 12								")
	fmt.Println("																												")
	fmt.Println("to host the worker node, run ./scripts/installService.bat as Administrator 									")
}