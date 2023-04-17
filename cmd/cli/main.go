package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/thinkonmay/thinkshare-daemon/credential"
)


type RunType int
const (
	failed = -1
	short_task = 0
)


func main() {
	task,err := ShortTask()
	switch int(task) {
	case failed:
		fmt.Printf("failed : %s\n",err.Error())
		return
	}
}





func ShortTask() (RunType,error) {
	out := ""

	for i,arg := range os.Args[1:]{ switch arg {
	case "proxy" :
		for _,arg1 := range os.Args[i:]{ switch arg1 {
			case "gen" :
			case "current" :
				credential.UseProxyAccount()
			case "reset" :
				os.Remove(credential.ProxySecretFile)
			}
		}

		proxy_account,err := credential.SetupProxyAccount()
		if err != nil { return failed,err }
		out = fmt.Sprintf("proxy account generated : %s",proxy_account.Username)
	case "vendor" :
		api_key,err := credential.SetupApiKey()
		if err != nil { return failed,err }

		for t,arg1 := range os.Args[i:]{ switch arg1 {
		case "keygen" :
			out = fmt.Sprintf("api key : %s",api_key.Key)
		case  "list-workers" :
			out,err = credential.FetchWorker(api_key)
			if err != nil { return failed,err }
		case  "create-session" :
			id,soundcard,monitor := -1,"Default Audio Render Device","Generic PnP Monitor"
			for v,arg2 := range os.Args[t:]{ switch arg2{
			case  "--worker-id" :
				if id,err = strconv.Atoi(os.Args[t:][v+1]);err != nil { panic(err) }
			case  "--monitor" :
				monitor = os.Args[t:][v+1]
			case  "--soundcard" :
				soundcard = os.Args[t:][v+1]
			}}

			out,err = credential.CreateSession(credential.Filter{
				WorkerId: id,
				SoudcardName: soundcard,
				MonitorName: monitor},api_key)
			if err != nil { return failed,err }
		case  "deactivate-session" :
			id := -1
			for v,arg2 := range os.Args[t:]{ switch arg2 {
			case  "--session-id" :
				if id,err = strconv.Atoi(os.Args[t:][v+1]);err != nil { panic(err) }
			} }

			out,err = credential.DeactivateSession(id,api_key)
			if err != nil { return failed,err }
		}}
	case "--help":
		printHelp()
	case "-h":
		printHelp()
	case "help":
		printHelp()
	}}


	if out != "" {
		fmt.Println(out)
		secret_f, err := os.OpenFile("./task_result.yaml", os.O_RDWR|os.O_CREATE, 0755)
		if err != nil { panic(err) }
		defer secret_f.Close()

		secret_f.Truncate(0)
		secret_f.WriteAt([]byte(out),0)
		return short_task,nil
	}

	return short_task,nil
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