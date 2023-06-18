package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/thinkonmay/thinkshare-daemon/cmd/cli/oauth2l"
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

			proxy_account, err := RegisterProxyAccount()
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

func RegisterProxyAccount() (account credential.Account, err error) {
	secret_f, err := os.OpenFile(credential.ProxySecretFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return credential.Account{}, err
	}

	defer func() {
		defer secret_f.Close()
		bytes, _ := json.MarshalIndent(account, "", "	")

		secret_f.Truncate(0)
		secret_f.WriteAt(bytes, 0)
	}()

	content, _ := io.ReadAll(secret_f)
	err = json.Unmarshal(content, &account)
	if err == nil && account.Username != nil {
		return account, nil
	}

	oauth2_code, err := oauth2l.StartAuth(credential.Secrets.Google.ClientId, 3000)
	if err != nil {
		return credential.Account{}, err
	}

	b, _ := json.Marshal(credential.Addresses)
	req, err := http.NewRequest("POST", credential.Secrets.EdgeFunctions.ProxyRegister, bytes.NewBuffer(b))
	if err != nil {
		return credential.Account{}, err
	}

	req.Header.Set("oauth2", oauth2_code)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", credential.Secrets.Secret.Anon))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return credential.Account{}, err
	}

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		body_str := string(body)
		return credential.Account{}, fmt.Errorf("response code %d: %s", resp.StatusCode, body_str)
	}

	if err := json.Unmarshal(body, &account); err != nil {
		return credential.Account{}, err
	}

	return
}
