package credential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/thinkonmay/thinkshare-daemon/credential/oauth2"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)



const (
	SecretFile = "./secret.json"
)

type Account struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Project  string `json:"project"`
}
type Address struct {
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
}

type Secret struct {
	EdgeFunctions struct {
		ProxyRegister           string `json:"proxy_register"`
		SessionAuthenticate     string `json:"session_authenticate"`
		SignalingAuthenticate   string `json:"signaling_authenticate"`
		TurnRegister            string `json:"turn_register"`
		WorkerProfileFetch      string `json:"worker_profile_fetch"`
		WorkerRegister          string `json:"worker_register"`
		WorkerSessionCreate     string `json:"worker_session_create"`
		WorkerSessionDeactivate string `json:"worker_session_deactivate"`
	} `json:"edge_functions"`

	Secret struct {
		Anon			string `json:"anon"`
		Url 			string `json:"url"`
    } `json:"secret"` 

    Google struct {
		ClientId       string `json:"client_id"`
    } `json:"google"`

    Conductor struct {
		Hostname string `json:"host"`
		GrpcPort int    `json:"grpc_port"`
    } `json:"conductor"`
}

var Secrets *Secret
var address *Address
var proj string = os.Getenv("PROJECT")

func init() {
	address = &Address{
		PublicIP:  system.GetPublicIPCurl(),
		PrivateIP: system.GetPrivateIP(),
	}

	proj = "kczvtfaouddunjtxcemk"
	resp,err := http.DefaultClient.Post(fmt.Sprintf("https://%s.functions.supabase.co/constant",proj),"application/json",bytes.NewBuffer([]byte("{}")))
	if err != nil {
		panic(err)
	}
	

	data := make([]byte, 100000)
	n,_ := resp.Body.Read(data)

	Secrets = &Secret{}
	err = json.Unmarshal(data[:n],Secrets)
	if err != nil {
		panic(err)
	}
}


func SetupProxyAccount() (account Account, err error) {
	secret_f, err := os.OpenFile(SecretFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return Account{}, err
	} else {
		bytes := make([]byte, 1000)
		count, _ := secret_f.Read(bytes)
		err = json.Unmarshal(bytes[:count], &account)
		if err == nil {
			return account, nil
		}
	}
	defer func ()  {
		account.Project = proj
		defer secret_f.Close()
		bytes, err := json.MarshalIndent(account, "", "")
		if err != nil { return }
		if _, err = secret_f.Write(bytes); err != nil {
			fmt.Printf("%s\n", err.Error())
		}
	}()




	oauth2_code, err := oauth2l.StartAuth(Secrets.Google.ClientId,3000)
	if err != nil {
		return Account{}, err
	}

	
	b, _ := json.Marshal(address)
	req, err := http.NewRequest("POST", Secrets.EdgeFunctions.ProxyRegister, bytes.NewBuffer(b))
	if err != nil {
		return Account{}, err
	}

	req.Header.Set("oauth2-token", oauth2_code)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Account{}, err
	}


	body := make([]byte, 10000)
	size, _ := resp.Body.Read(body)
	if resp.StatusCode != 200 {
		body_str := string(body[:size])
		return Account{}, fmt.Errorf("response code %d: %s", resp.StatusCode, body_str)
	}

	if err := json.Unmarshal(body[:size], &account); err != nil {
		return Account{}, err
	}

	return account, nil
}

func SetupWorkerAccount(proxy Account) (
						cred Account,
						err error) {

	b, _ := json.Marshal(address)
	req, err := http.NewRequest("POST", Secrets.EdgeFunctions.WorkerRegister, bytes.NewBuffer(b))
	if err != nil {
		return Account{}, err
	}

	req.Header.Set("username", proxy.Username)
	req.Header.Set("password", proxy.Password)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Account{}, err
	}

	body := make([]byte, 10000)
	size, _ := resp.Body.Read(body)
	if resp.StatusCode != 200 {
		body_str := string(body[:size])
		return Account{}, fmt.Errorf("response code %d: %s", resp.StatusCode, body_str)
	}

	if err := json.Unmarshal(body[:size], &proxy); err != nil {
		return Account{}, err
	}

	return proxy, nil
}
