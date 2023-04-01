package credential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)



const (
	secret_file = "./secret.json"
)

type Account struct {
	Username string `json:"username"`
	Password string `json:"password"`
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
}

var secret *Secret

func init() {
	proj := os.Getenv("PROJECT")
	proj = "avmvymkexjarplbxwlnj"
	resp,err := http.DefaultClient.Post(fmt.Sprintf("https://%s.functions.supabase.co/constant",proj),"application/json",bytes.NewBuffer([]byte("{}")))
	if err != nil {
		panic(err)
	}
	

	data := make([]byte, 100000)
	n,_ := resp.Body.Read(data)

	secret = &Secret{}
	err = json.Unmarshal(data[:n],secret)
	if err != nil {
		panic(err)
	}
}


func SetupProxyAccount(addr Address) (cred Account, err error) {
	secret, err := os.OpenFile(secret_file, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return Account{}, err
	} else {
		bytes := make([]byte, 1000)
		count, _ := secret.Read(bytes)
		err = json.Unmarshal(bytes[:count], &cred)
		if err == nil {
			return cred, nil
		}
	}
	defer func ()  {
		defer secret.Close()
		bytes, err := json.MarshalIndent(cred, "", "")
		if err != nil { return }
		if _, err = secret.Write(bytes); err != nil {
			fmt.Printf("%s\n", err.Error())
		}
	}()




	// oauth2_code, err := oauth2l.StartAuth(sysinf)
	if err != nil {
		return Account{}, err
	}



	return cred, nil
}

func SetupWorkerAccount(data Address,
						proxy Account) (
						cred *Account,
						err error) {

	b, _ := json.Marshal(data)
	req, err := http.NewRequest("POST", secret.EdgeFunctions.WorkerRegister, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}

	req.Header.Set("username", proxy.Username)
	req.Header.Set("password", proxy.Password)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", secret.Secret.Anon))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	body := make([]byte, 10000)
	size, _ := resp.Body.Read(body)
	if resp.StatusCode != 200 {
		body_str := string(body[:size])
		return nil, fmt.Errorf("response code %d: %s", resp.StatusCode, body_str)
	}

	if err := json.Unmarshal(body[:size], &proxy); err != nil {
		return nil, err
	}

	return &proxy, nil
}
