package credential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

const (
	SecretDir 				= "./secret"
	ProxySecretFile 		= "./secret/proxy.json"
	StorageCred 			= "/.credential.thinkmay.json"

	API_VERSION				= "v2"
	PROJECT 	 			= "supabase.thinkmay.net"
	ANON_KEY 				= "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ewogICJyb2xlIjogImFub24iLAogICJpc3MiOiAic3VwYWJhc2UiLAogICJpYXQiOiAxNjk0MDE5NjAwLAogICJleHAiOiAxODUxODcyNDAwCn0.EpUhNso-BMFvAJLjYbomIddyFfN--u-zCf0Swj9Ac6E"


	NEED_WAIT               = 8
)

func GetStorageCredentialFile(mountpoint string) string {
	return fmt.Sprintf("%s%s", mountpoint, StorageCred)
}

type Account struct {
	Username *string `json:"username"`
	Password *string `json:"password"`
}

var Addresses = &struct {
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
}{}

func init() {
	retry := 0 
	for {
		Addresses.PublicIP  = system.GetPublicIPCurl()
		Addresses.PrivateIP = system.GetPrivateIP()
		if  Addresses.PrivateIP != "" && Addresses.PublicIP != "" {
			break
		} else if retry == 10 {
			panic("server is not connected to the internet")
		}
		time.Sleep(10 * time.Second)
		retry = retry + 1
	}
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
	req, err := http.NewRequest(
		"POST", 
		fmt.Sprintf("https://%s/functions/%s/worker_register",PROJECT,API_VERSION), 
		bytes.NewBuffer(b))
	if err != nil {
		return Account{}, err
	}

	req.Header.Set("username", *proxy.Username)
	req.Header.Set("password", *proxy.Password)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ANON_KEY))

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

func ReadOrRegisterStorageAccount( worker Account,
								  partition *packet.Partition,
								) (storage *Account,
								   err error,
								   abort bool,
								   ) {
	path := GetStorageCredentialFile(partition.Mountpoint)
	secret_f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return nil, fmt.Errorf("unable to readfie %s",err.Error()),false
	}

	var body []byte
	storage = &Account{}
	data, _ := io.ReadAll(secret_f)
	if len(data) == 0 {
		body,_ = json.Marshal(struct {
			Hardware *packet.Partition `json:"hardware"`
			AccessPoint *struct {
				PublicIP  string `json:"public_ip"`
				PrivateIP string `json:"private_ip"`
			} `json:"access_point"`
		}{
			Hardware: partition,
			AccessPoint: Addresses,
		})
	} else {
		err = json.Unmarshal(data, storage)
		if err != nil {
			return nil, err,false
		}

		body,_ = json.Marshal(struct {
			Storage *Account `json:"storage"`
		}{
			Storage: storage,
		})
	}

	defer func() {
		if abort { defer os.Remove(path) } 
		defer secret_f.Close()
		if err != nil || abort { return } 

		bytes, _ := json.MarshalIndent(storage, "", "	")
		secret_f.Truncate(0)
		secret_f.WriteAt(bytes, 0)
	}()


	req, err := http.NewRequest("POST", 
		fmt.Sprintf("https://%s/functions/%s/storage_register",PROJECT,API_VERSION), 
		bytes.NewBuffer(body))
	if err != nil {
		return nil, err,false
	}

	req.Header.Set("username", *worker.Username)
	req.Header.Set("password", *worker.Password)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ANON_KEY))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err,false
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, err,false
	} else if resp.StatusCode != 200 {
		errcode := &struct{
			Message *string `json:"message"`
			Code *int `json:"code"`
		}{
			Message: nil,
			Code:nil,
		}

		err = json.Unmarshal(data, errcode)
		if err != nil {
			return nil,err,false
		} else if (*errcode.Code == NEED_WAIT) {
			return nil,fmt.Errorf(*errcode.Message),false
		} else {
			return nil,fmt.Errorf("%s",*errcode.Message),true
		}
	} else {
		storage = &Account{}
		err = json.Unmarshal(data, storage)
		if err != nil {
			return nil, err,false
		} else {
			return storage,nil,false
		}
	}
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
		fmt.Sprintf("https://%s/functions/%s/turn_register",PROJECT,API_VERSION), 
		bytes.NewBuffer(b))
	if err != nil {
		return 
	}

	req.Header.Set("username", *proxy.Username)
	req.Header.Set("password", *proxy.Password)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ANON_KEY))

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

	req,err := http.NewRequest(
		"POST",
		fmt.Sprintf("https://%s/rest/v1/rpc/ping_account",PROJECT),
		bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ANON_KEY)
	req.Header.Set("apikey",ANON_KEY)
	resp,err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		data,_ := io.ReadAll(resp.Body)
		return fmt.Errorf(string(data))
	}

	return
}