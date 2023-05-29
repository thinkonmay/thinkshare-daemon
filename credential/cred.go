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
	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

const (
	SecretDir       = "./secret"

	ProxySecretFile = "./secret/proxy.json"
	ConfigFile      = "./secret/config.json"

	StorageCred     = "/.thinkmay/credential.json"
)

func GetStorageCredentialFile(mountpoint string) string {
	return fmt.Sprintf("%s%s",mountpoint,StorageCred)
}



type Account struct{
	Username string `json:"username"`
	Password string `json:"password"`
}

var Secrets = &struct{
	EdgeFunctions struct {
		UserKeygen              string `json:"user_keygen"`
		ProxyRegister           string `json:"proxy_register"`
		SessionAuthenticate     string `json:"session_authenticate"`
		SignalingAuthenticate   string `json:"signaling_authenticate"`
		TurnRegister            string `json:"turn_register"`
		WorkerProfileFetch      string `json:"worker_profile_fetch"`
		WorkerRegister          string `json:"worker_register"`
		StorageRegister         string `json:"storage_register"`
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
		GrpcPort int    `json:"port"`
	} `json:"conductor"`
}{}

var Addresses = &struct{
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
}{
	PublicIP:  system.GetPublicIPCurl(),
	PrivateIP: system.GetPrivateIP(),
}



// TODO update version
func init() {
	proj := os.Getenv("PROJECT")
	if proj == "" {
		proj = "avmvymkexjarplbxwlnj"
	}
	commitHash, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err == nil {
		fmt.Printf("current commit hash: %s \n", commitHash)
	} else if commitHash == nil {
		fmt.Println("you are not using git, please download git to have auto update")
	} else if strings.Contains(string(commitHash), "fatal") {
		fmt.Println("you did not clone this repo, please use clone")
	}

	os.Mkdir(SecretDir, os.ModeDir)
	secretFile, err := os.OpenFile(ConfigFile, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		panic(err)
	}
	defer func() {
		defer secretFile.Close()
		bytes, _ := json.MarshalIndent(Secrets, "", "	")

		secretFile.Truncate(0)
		secretFile.WriteAt(bytes, 0)
	}()

	data, _ := io.ReadAll(secretFile)
	err = json.Unmarshal(data, Secrets)

	if err == nil { return } // avoid fetch if there is already secrets

	body,_ := json.Marshal(Addresses)
	resp, err := http.DefaultClient.Post(fmt.Sprintf("https://%s.functions.supabase.co/constant", proj), "application/json", bytes.NewBuffer(body))
	if err != nil {
		panic(err)
	} else if resp.StatusCode != 200 {
		panic("unable to fetch constant from server")
	}

	body,_ = io.ReadAll(resp.Body)
	json.Unmarshal(body, Secrets)
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
	fmt.Printf ("credential : ")

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

	if err := json.Unmarshal(body, &cred); err != nil {
		return Account{}, err
	}

	return
}


func ReadOrRegisterStorageAccount(worker Account,
								  partition *packet.Partition,
								  ) (account Account, 
									 err error) {
	secret_f, err := os.OpenFile(GetStorageCredentialFile(partition.Mountpoint), os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return Account{}, err
	}

	data, _ := io.ReadAll(secret_f)
	err = json.Unmarshal(data, &account)
	if err != nil && account.Username != "" && account.Password != "" {
		return account, err
	}


	defer func() {
		defer secret_f.Close()
		if err != nil {
			return 
		}

		bytes, _ := json.MarshalIndent(account, "", "	")
		secret_f.Truncate(0)
		secret_f.WriteAt(bytes, 0)
	}()


	data,_ = json.Marshal(partition)
	req, err := http.NewRequest("POST", Secrets.EdgeFunctions.StorageRegister, bytes.NewBuffer(data))
	if err != nil {
		return Account{}, err
	}

	req.Header.Set("username", worker.Username)
	req.Header.Set("password", worker.Password)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", Secrets.Secret.Anon))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Account{}, err
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return Account{}, err
	}

	if resp.StatusCode != 200 {
		return Account{}, fmt.Errorf("response code %d: %s", resp.StatusCode, string(data))
	}
	err = json.Unmarshal(data, &account)
	if err != nil {
		return Account{}, err
	}


	return
}