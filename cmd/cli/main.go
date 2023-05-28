package main

import (
	"fmt"
	"os"

	"github.com/thinkonmay/thinkshare-daemon/credential"
)


func main() {
	err := ShortTask()
	if err != nil {
		fmt.Printf("failed : %s\n", err.Error())
	}
}

func ShortTask() (error) {
	for i, arg := range os.Args[1:] {
		switch arg {
		case "proxy":
			for _, arg1 := range os.Args[i:] {
				switch arg1 {
				case "gen":
				case "provide":
					credential.InputProxyAccount()
				case "reset":
					os.Remove(credential.ProxySecretFile)
				}
			}

			proxy_account, err := credential.RegisterProxyAccount()
			if err != nil {
				return err
			}
			fmt.Printf("proxy account generated : %s", proxy_account.Username)
		case "--help":
			printHelp()
		case "-h":
			printHelp()
		case "help":
			printHelp()
		}
	}

	return nil
}

func printHelp() {
	fmt.Println("required environment (always): 		")
	fmt.Println(" - PROJECT: project id ")
	fmt.Println("")
	fmt.Println("A. CLI to do simple tasks, format: ./daemon.exe proxy current")
	fmt.Println(" -proxy			")
	fmt.Println("    -current              (when) get current proxy account username (generate if none)	")
	fmt.Println("    -reset                (when) get new proxy account	")
	fmt.Println("B. Run the worker node   ")
	fmt.Println("     1. ./daemon.exe")
	fmt.Println("     2. run ./scripts/installService.bat as Administrator ")
}
