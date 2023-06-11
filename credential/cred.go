package credential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	oauth2l "github.com/thinkonmay/thinkshare-daemon/credential/oauth2"
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

const (
	SecretDir = "./secret"
	ProxySecretFile = "./secret/proxy.json"
	StorageCred = "/.credential.thinkmay.json"
)

func GetStorageCredentialFile(mountpoint string) string {
	return fmt.Sprintf("%s%s", mountpoint, StorageCred)
}

type Account struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
}

var Secrets = &struct {
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
		StorageRegister        	string `json:"storage_register"`
		UserApplicationFetch  	string `json:"user_application_fetch"`
		RequestApplication     	string `json:"request_application"`
		FetchWorkerStatus     	string `json:"fetch_worker_status"`
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
		GrpcPort int    `json:"port"`
	} `json:"conductor"`

	Daemon struct {
		Commit string `json:"commit"`
	} `json:"daemon"`



	Supabase *struct {
		Url              		string `json:"url"`
		AnonKey         		string `json:"anon_key"`
		ServiceRoleKey 			string `json:"admin_key"`
		DbConn					string `json:"db_conn"`
	} `json:"supabase"`

	Elasticsearch *struct {
		Url      			string `json:"url"`
		Namespace			string `json:"namespace"`
	} `json:"elasticSearch"`

	Signaling *struct {
		HostName					string 	`json:"HostName"`
		ValidateUrl 				string 	`json:"ValidationUrl"`

		Data struct {
			GrpcPort      			int 	`json:"GrpcPort"`
			Path					string  `json:"Path"`
		} `json:"Data"`
		Video struct {
			GrpcPort      			int 	`json:"GrpcPort"`
			Path					string  `json:"Path"`
		} `json:"Video"`
		Audio struct {
			GrpcPort      			int 	`json:"GrpcPort"`
			Path					string  `json:"Path"`
		} `json:"Audio"`
	}`json:"signalingSetting"`

	ConductorSetting *struct {
		GrpcPort 	int    `json:"Port"`
		HostName    string `json:"HostName"`
		Commit   	string `json:"commit"`
	}`json:"conductorSetting"`
}{}

var Addresses = &struct {
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
}{
	PublicIP:  system.GetPublicIPCurl(),
	PrivateIP: system.GetPrivateIP(),
}

func SetupEnv(proj string,anon_key string) {
	os.Mkdir(SecretDir, os.ModeDir)
	req,err := http.NewRequest("GET",
		fmt.Sprintf("https://%s.supabase.co/rest/v1/constant?select=value", proj),
		bytes.NewBuffer([]byte("")))
	if err != nil {
		panic(err)
	}

	req.Header.Set("apikey", anon_key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s",anon_key))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	} else if resp.StatusCode != 200 {
		panic("unable to fetch constant from server")
	}

	body, _ := io.ReadAll(resp.Body)

	var data [](interface{})
	err = json.Unmarshal(body, &data)
	if err != nil {
		panic(err)
	}

	val,_ := json.Marshal(data[0].(map[string]interface{})["value"])
	json.Unmarshal(val, &Secrets)
}

func InputProxyAccount() (account Account, err error) {
	secret_f, err := os.OpenFile(ProxySecretFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return Account{}, err
	}

	bytes, _ := io.ReadAll(secret_f)
	err = json.Unmarshal(bytes, &account)
	if err == nil {
		secret_f.Close()
		return account, nil
	}

	fmt.Println("paste your proxy credential here (which have been copied to your clipboard)")
	fmt.Println("- to register proxy account, go to https://thinkmay.net/ , open terminal application and run proxy register")
	fmt.Printf("credential : ")

	text := "{}"
	fmt.Scanln(&text)
	json.Unmarshal([]byte(text), &account)

	defer func() {
		defer secret_f.Close()
		bytes, _ := json.MarshalIndent(account, "", "	")

		secret_f.Truncate(0)
		secret_f.WriteAt(bytes, 0)
	}()

	return account, nil
}

func RegisterProxyAccount() (account Account, err error) {
	secret_f, err := os.OpenFile(ProxySecretFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return Account{}, err
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

	return
}

func SetupWorkerAccount(proxy Account) (
	cred Account,
	err error) {

	b, _ := json.Marshal(Addresses)
	req, err := http.NewRequest("POST", Secrets.EdgeFunctions.WorkerRegister, bytes.NewBuffer(b))
	if err != nil {
		return Account{}, err
	}

	req.Header.Set("username", *proxy.Username)
	req.Header.Set("password", *proxy.Password)
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

	if err := json.Unmarshal(body, &cred); err != nil {
		return Account{}, err
	}

	return
}

func ReadOrRegisterStorageAccount(proxy Account,
								  worker Account,
								  partition *packet.Partition,
								) (storage *Account,
								   err error) {
	path := GetStorageCredentialFile(partition.Mountpoint)
	secret_f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, err
	}

	data, _ := io.ReadAll(secret_f)

	storage = &Account{}
	err = json.Unmarshal(data, storage)
	if err == nil && storage.Username != nil && storage.Password != nil {
		storage = nil
	}

	do_save := true
	defer func() {
		defer func ()  {
			secret_f.Close()
			if !do_save {
				os.Remove(path)
			}
		}() 
		if err != nil || storage == nil || !do_save {
			return
		} else if storage.Username == nil || storage.Password == nil {
			return
		}

		bytes, _ := json.MarshalIndent(storage, "", "	")
		secret_f.Truncate(0)
		secret_f.WriteAt(bytes, 0)
	}()



	data, _ = json.Marshal(struct {
		Proxy Account `json:"proxy"`
		Worker Account `json:"worker"`
		Storage *Account `json:"storage,omitempty"`
		Hardware *packet.Partition `json:"hardware"`
		AccessPoint *struct {
			PublicIP  string `json:"public_ip"`
			PrivateIP string `json:"private_ip"`
		} `json:"access_point"`
	}{
		Proxy: proxy,
		Worker: worker,
		Storage: storage,
		Hardware: partition,
		AccessPoint: Addresses,
	})

	req, err := http.NewRequest("POST", 
		Secrets.EdgeFunctions.StorageRegister, 
		bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("response code %d: %s", resp.StatusCode, string(data))
	} 

	if string(data) == "\"NOT_REGISTER\"" {
		fmt.Println("aborted storage credential save")
		do_save = false
		return &Account{},nil
	}

	storage = &Account{}
	err = json.Unmarshal(data, storage)
	if err != nil {
		return nil, err
	}

	return
}