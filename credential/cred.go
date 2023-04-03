package credential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	oauth2l "github.com/thinkonmay/thinkshare-daemon/credential/oauth2"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
	"gopkg.in/yaml.v3"
)

const (
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
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
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

var Secrets *Secret
var Addresses *Address
var proj string = os.Getenv("PROJECT")

func init() {
	os.Mkdir("./secret",os.ModeDir)

	Secrets = &Secret{}
	Addresses = &Address{
		PublicIP:  system.GetPublicIPCurl(),
		PrivateIP: system.GetPrivateIP(),
	}

	secret_f, err := os.OpenFile(ConfigFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	defer secret_f.Close()

	data, _ := io.ReadAll(secret_f)
	err = json.Unmarshal(data, Secrets)
	if err != nil {
		resp, err := http.DefaultClient.Post(fmt.Sprintf("https://%s.functions.supabase.co/constant", proj), "application/json", bytes.NewBuffer([]byte("{}")))
		if err != nil {
			panic(err)
		}

		body,_ := io.ReadAll(resp.Body)
		err = json.Unmarshal(body, Secrets)
		if err != nil {
			panic(err)
		}

		secret_f.Truncate(0)
		bytes, _ := json.MarshalIndent(Secrets, "", "	")
		secret_f.WriteAt(bytes, 0)
	}
}

func UseProxyAccount() (account Account, err error) {
	secret_f, err := os.OpenFile(ProxySecretFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return Account{}, err
	}

	defer secret_f.Close()
	bytes,_ := io.ReadAll(secret_f)
	err = json.Unmarshal(bytes, &account)
	if err != nil {
		return Account{}, fmt.Errorf("nil proxy account")
	}

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
		if _, err = secret_f.WriteAt(bytes, 0); err != nil {
			fmt.Printf("%s\n", err.Error())
		}
	}()

	content,_ := io.ReadAll(secret_f)
	err = json.Unmarshal(content, &account)
	if err == nil {
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
		if _, err = secret_f.WriteAt(bytes, 0); err != nil {
			fmt.Printf("%s\n", err.Error())
		}
	}()

	content, _ := io.ReadAll(secret_f)
	err = json.Unmarshal(content, &cred)
	if err == nil {
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




func FetchWorker(cred ApiKey) (result string, err error) {
	body,_ := json.Marshal(struct{
		OnlyActive bool `json:"only_active"`
	}{
		OnlyActive: false,
	})

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