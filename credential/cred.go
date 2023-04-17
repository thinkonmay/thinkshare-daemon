package credential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	oauth2l "github.com/thinkonmay/thinkshare-daemon/credential/oauth2"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
	"gopkg.in/yaml.v3"
)

const (
	SecretDir		= "./secret"
	ProxySecretFile = "./secret/proxy.json"
	UserSecretFile  = "./secret/user.json"
	ConfigFile 		= "./secret/config.json"
)

type ApiKey struct {
	Key     string `json:"key"`
	Project string `json:"project"`
}
type Account struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Project  string `json:"project"`
}
type Address struct {
	PublicIP   string `json:"public_ip"`
	PrivateIP  string `json:"private_ip"`
	CommitHash string `json:"commit"`
}

type Secret struct {
	EdgeFunctions struct {
		UserKeygen              string `json:"user_keygen"`
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
		Anon string `json:"anon"`
		Url  string `json:"url"`
	} `json:"secret"`

	Google struct {
		ClientId string `json:"client_id"`
	} `json:"google"`

	Conductor struct {
		Hostname string `json:"host"`
		GrpcPort int    `json:"grpc_port"`
	} `json:"conductor"`
}

var Secrets *Secret = &Secret{}
var proj string = os.Getenv("PROJECT")
var Addresses *Address = &Address{
	PublicIP:  system.GetPublicIPCurl(),
	PrivateIP: system.GetPrivateIP(),
	CommitHash: "unknown",
}

func init() {
	if proj == "" { proj = "avmvymkexjarplbxwlnj" }
	commitHash,err := exec.Command("git","rev-parse","HEAD").Output()
	if err == nil {
		fmt.Printf("current commit hash: %s \n",commitHash)
		Addresses.CommitHash = string(commitHash)
	} else if commitHash == nil {
		fmt.Println("you are not using git, please download git to have auto update")
	} else if strings.Contains(string(commitHash),"fatal") {
		fmt.Println("you did not clone this repo, please use clone")
	}

	os.Mkdir(SecretDir,os.ModeDir)
	secretFile, err := os.OpenFile(ConfigFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	defer  func ()  {
		defer secretFile.Close()
		bytes,_ := json.MarshalIndent(Secrets, "", "	")
		secretFile.Truncate(0)
		secretFile.WriteAt(bytes, 0)
	}()

	data,_ := io.ReadAll(secretFile)
	err = json.Unmarshal(data, Secrets)

	if err == nil { return } // avoid fetch if there is already secrets
	resp, err := http.DefaultClient.Post(fmt.Sprintf("https://%s.functions.supabase.co/constant", proj), "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		fmt.Printf("unable to fetch constant from server %s\n",err.Error())
		return
	} else if resp.StatusCode != 200 {
		fmt.Println("unable to fetch constant from server")
		return
	}

	body,_ := io.ReadAll(resp.Body)
	json.Unmarshal(body, Secrets)
}

func UseProxyAccount() (account Account, err error) {
	secret_f, err := os.OpenFile(ProxySecretFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return Account{}, err
	}

	bytes,_ := io.ReadAll(secret_f)
	err = json.Unmarshal(bytes, &account)
	if err != nil {
		fmt.Println("none proxy account provided, please provide (look into ./secret folder on the machine you setup proxy account)")
		fmt.Printf("username : ")
		fmt.Scanln(&account.Username)
		fmt.Printf("password : ")
		fmt.Scanln(&account.Password)
		account.Project = proj

		defer func() {
			bytes, _ := json.MarshalIndent(account, "", "	")
			secret_f.Truncate(0)
			secret_f.WriteAt(bytes, 0)
			secret_f.Close()
		}()

		return account,nil
	}

	secret_f.Close()
	return account, nil
}

func SetupProxyAccount() (account Account, err error) {
	secret_f, err := os.OpenFile(ProxySecretFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return Account{}, err
	}

	defer func() {
		account.Project = proj
		defer secret_f.Close()
		bytes, _ := json.MarshalIndent(account, "", "	")

		secret_f.Truncate(0)
		secret_f.WriteAt(bytes, 0)
	}()

	content,_ := io.ReadAll(secret_f)
	err = json.Unmarshal(content, &account)
	if err == nil && account.Username != "" {
		return account, nil
	}

	oauth2_code, err := oauth2l.StartAuth(Secrets.Google.ClientId, 3000)
	if err != nil {
		return Account{}, err
	}

	b, _ := json.Marshal(Addresses)
	req, err := http.NewRequest("POST", Secrets.EdgeFunctions.ProxyRegister, bytes.NewBuffer(b))
	if err != nil {
		return Account{}, err
	}

	req.Header.Set("oauth2", oauth2_code)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Account{}, err
	}

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		body_str := string(body)
		return Account{}, fmt.Errorf("response code %d: %s", resp.StatusCode, body_str)
	}

	if err := json.Unmarshal(body, &account); err != nil {
		return Account{}, err
	}

	return account, nil
}

func SetupWorkerAccount(proxy Account) (
	cred Account,
	err error) {

	b, _ := json.Marshal(Addresses)
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

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		body_str := string(body)
		return Account{}, fmt.Errorf("response code %d: %s", resp.StatusCode, body_str)
	}

	if err := json.Unmarshal(body, &proxy); err != nil {
		return Account{}, err
	}

	return proxy, nil
}

func SetupApiKey() (cred ApiKey,
	err error) {
	secret_f, err := os.OpenFile(UserSecretFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return ApiKey{}, err
	}

	defer func() {
		cred.Project = proj
		defer secret_f.Close()
		bytes, _ := json.MarshalIndent(cred, "", "	")

		secret_f.Truncate(0)
		secret_f.WriteAt(bytes, 0)
	}()

	content, _ := io.ReadAll(secret_f)
	err = json.Unmarshal(content, &cred)
	if err == nil && cred.Key != "" {
		return cred, nil
	}

	oauth2_code, err := oauth2l.StartAuth(Secrets.Google.ClientId, 3000)
	if err != nil {
		return ApiKey{}, err
	}

	b, _ := json.Marshal(Addresses)
	req, err := http.NewRequest("POST", Secrets.EdgeFunctions.UserKeygen, bytes.NewBuffer(b))
	if err != nil {
		return ApiKey{}, err
	}

	req.Header.Set("oauth2", oauth2_code)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ApiKey{}, err
	}

	content, _ = io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		body_str := string(content)
		return ApiKey{}, fmt.Errorf("response code %d: %s", resp.StatusCode, body_str)
	}

	if err := json.Unmarshal(content, &cred); err != nil {
		return ApiKey{}, err
	}

	return cred, nil
}




func FetchWorker(cred ApiKey, worker_ip *string) (result string, err error) {
	data := struct{
		UseCase string `json:"use_case"`
		WaitFor *struct{
			WorkerIp string `json:"worker_ip"`
		}`json:"wait_for"`
	}{
		UseCase: "cli",
	}

	if worker_ip != nil {
		data.WaitFor = &struct{
			WorkerIp string "json:\"worker_ip\"" 
		}{
			WorkerIp: "abc",
		}
	}


	body,_ := json.Marshal(data)
	req, err := http.NewRequest("POST", Secrets.EdgeFunctions.WorkerProfileFetch, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("api_key", cred.Key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	content,_ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		body_str := string(content)
		return "", fmt.Errorf("response code %d: %s", resp.StatusCode, body_str)
	}

	var stuff interface{}
	if err := json.Unmarshal(content, &stuff); err != nil {
		return "", err
	}

	val,_ := yaml.Marshal(stuff)
	return string(val), nil
}



type Filter struct {
  	WorkerId 	 int	`json:"worker_id"`
	MonitorName  string `json:"monitor_name"`
	SoudcardName string `json:"soudcard_name"`
}


func CreateSession(filter Filter,cred ApiKey) (out string, err error) {
	body,_ := json.Marshal(filter)

	req, err := http.NewRequest("POST", Secrets.EdgeFunctions.WorkerSessionCreate, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("api_key", cred.Key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	content,_ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		body_str := string(content)
		return "", fmt.Errorf("response code %d: %s", resp.StatusCode, body_str)
	}

	var stuff interface{}
	if err := json.Unmarshal(content, &stuff); err != nil {
		return "", err
	}

	val,err := yaml.Marshal(stuff)
	return string(val),err 
}

func DeactivateSession(SessionID int,cred ApiKey) (URL string, err error) {
	body,_ := json.Marshal(struct{
		WorkerSessionId int `json:"worker_session_id"`
	}{
		WorkerSessionId: SessionID,
	})

	req, err := http.NewRequest("POST", Secrets.EdgeFunctions.WorkerSessionDeactivate, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}

	req.Header.Set("api_key", cred.Key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	content,_ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("response code %d: %s", resp.StatusCode, string(content))
	}

	var stuff interface{}
	if err := json.Unmarshal(content, &stuff); err != nil {
		return "", err
	}

	val,err := yaml.Marshal(stuff)
	return string(val), err
}