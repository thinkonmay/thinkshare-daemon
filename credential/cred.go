package credential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"

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
		ProxyRegister           string `json:"proxy_register"`
		TurnRegister            string `json:"turn_register"`
		WorkerRegister          string `json:"worker_register"`
		StorageRegister        	string `json:"storage_register"`

		SessionAuthenticate     string `json:"session_authenticate"`
		SignalingAuthenticate   string `json:"signaling_authenticate"`

		WorkerProfileFetch      string `json:"worker_profile_fetch"`
		WorkerSessionCreate     string `json:"worker_session_create"`
		WorkerSessionDeactivate string `json:"worker_session_deactivate"`

		UserApplicationFetch  	string `json:"user_application_fetch"`
		RequestApplication     	string `json:"request_application"`
	} `json:"edge_functions"`

	Secret struct {
		Url   string `json:"url"`
		Anon  string `json:"anon_key"`
		DbCon *string `json:"db_conn"`
		Admin *string `json:"admin_key"`
	} `json:"secret"`

	Conductor struct {
		Hostname string  `json:"host"`
		GrpcPort int     `json:"port"`
	} `json:"conductor"`

	Signaling *struct {
		Validate 					string 	`json:"Validation"`
		Video struct {
			Path					string  `json:"Path"`
		} `json:"Video"`
		Audio struct {
			Path					string  `json:"Path"`
		} `json:"Audio"`
	}`json:"signaling"`

	Daemon struct {
		Commit string `json:"commit"`
	} `json:"daemon"`
}{}

var Addresses = &struct {
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
}{
	PublicIP:  system.GetPublicIPCurl(),
	PrivateIP: system.GetPrivateIP(),
}

func SetupEnvAdmin(proj string,admin_key string) {
	req,err := http.NewRequest("GET",
		fmt.Sprintf("%s/rest/v1/constant?select=value&type=eq.ADMIN", proj),
		bytes.NewBuffer([]byte("")))
	if err != nil {
		panic(err)
	}

	req.Header.Set("apikey", admin_key)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s",admin_key))

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
func SetupEnv(proj string,anon_key string) {
	os.Mkdir(SecretDir, os.ModeDir)
	req,err := http.NewRequest("GET",
		fmt.Sprintf("%s/rest/v1/constant?select=value", proj),
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
								   readerr error,
								   registerr error,
								   ) {
	path := GetStorageCredentialFile(partition.Mountpoint)
	secret_f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, fmt.Errorf("unable to readfie %s",err.Error()),nil
	}

	body := []byte{}
	storage = &Account{}
	data, _ := io.ReadAll(secret_f)
	if len(data) == 0 {
		body,_ = json.Marshal(struct {
			Proxy Account `json:"proxy"`
			Worker Account `json:"worker"`
			Hardware *packet.Partition `json:"hardware"`
			AccessPoint *struct {
				PublicIP  string `json:"public_ip"`
				PrivateIP string `json:"private_ip"`
			} `json:"access_point"`
		}{
			Proxy: proxy,
			Worker: worker,
			Hardware: partition,
			AccessPoint: Addresses,
		})
	} else {
		err = json.Unmarshal(data, storage)
		if err != nil {
			return nil, err,nil
		}

		body,_ = json.Marshal(struct {
			Proxy Account `json:"proxy"`
			Worker Account `json:"worker"`
			Storage *Account `json:"storage"`
		}{
			Proxy: proxy,
			Worker: worker,
			Storage: storage,
		})
	}

	defer func() {
		if registerr != nil { defer os.Remove(path) } 
		defer secret_f.Close()
		if err != nil || registerr != nil { return } 

		bytes, _ := json.MarshalIndent(storage, "", "	")
		secret_f.Truncate(0)
		secret_f.WriteAt(bytes, 0)
	}()


	req, err := http.NewRequest("POST", 
		Secrets.EdgeFunctions.StorageRegister, 
		bytes.NewBuffer(body))
	if err != nil {
		return nil, err,nil
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err,nil
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil,err
	} else if resp.StatusCode != 200 {
		return nil, nil,fmt.Errorf("response code %d: %s", resp.StatusCode, string(data))
	} else if string(data) == "\"NOT_REGISTER\"" {
		return nil, nil,fmt.Errorf("aborted storage credential save")
	}

	storage = &Account{}
	err = json.Unmarshal(data, storage)
	if err != nil {
		return nil, nil,err
	}

	return storage,nil,nil
}

type TurnCred struct {
	Username string `json:"username"`
	Password string `json:"credential"`
}
type TurnResult struct {
	AccountID string   `json:"account_id"`
	Turn      TurnCred `json:"credential"`
}
type TurnInfo struct {
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
	Port      int    `json:"turn_port"`
	Scope     string `json:"scope"`
}

func GetFreeUDPPort(min int,max int) (int, error) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	port := l.LocalAddr().(*net.UDPAddr).Port
	if port > max {
		return 0,fmt.Errorf("invalid port %d",port)
	} else if port < min {
		return GetFreeUDPPort(min,max)
	}
	return port, nil
}



func SetupTurnAccount(proxy Account,
					 min int, 
					 max int) (
					 cred string,
					 turn TurnCred,
					 info TurnInfo,
					 err error) {
	port,_ := GetFreeUDPPort(min,max)
	info = TurnInfo{
		PublicIP: Addresses.PublicIP,
		PrivateIP: Addresses.PrivateIP,
		Port: port,
		Scope: "ip",
	}

	b, _ := json.Marshal(info)
	req, err := http.NewRequest("POST", 
		Secrets.EdgeFunctions.TurnRegister, 
		bytes.NewBuffer(b))
	if err != nil {
		return 
	}

	req.Header.Set("username", *proxy.Username)
	req.Header.Set("password", *proxy.Password)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 
	}

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		body_str := string(body)
        err = fmt.Errorf("response code %d: %s", resp.StatusCode, body_str)
        return
	}

	turn_account := TurnResult{}
	if err = json.Unmarshal(body, &turn_account); err != nil {
		return 
	}

	turn = turn_account.Turn
	cred = turn_account.AccountID

	return
}

func Ping(uid string)( err error)  {
	body,_ := json.Marshal( struct {
		AccountID string `json:"account_uid"`
	}{
		AccountID: uid,
	})

	req,err := http.NewRequest("POST",fmt.Sprintf("%s/rest/v1/rpc/ping_account",Secrets.Secret.Url),bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+Secrets.Secret.Anon)
	req.Header.Set("apikey",Secrets.Secret.Anon)
	resp,err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		data,_ := io.ReadAll(resp.Body)
		return fmt.Errorf(string(data))
	}

	return
}