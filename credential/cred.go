package credential

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/pigeatgarlic/oauth2l"
)

type Cred struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type Data struct {
	PublicIP  string `json:"public_ip"`
	PrivateIP string `json:"private_ip"`
}

func SetupProxyAccount(sysinf interface{}) (cred *Cred, err error) {
	cred = &Cred{}
	secret, err := os.OpenFile("./cache.secret.json", os.O_RDWR|os.O_CREATE, 0755)
	defer func() {
		if err := secret.Close(); err != nil {
			fmt.Printf("%s", err.Error())
		}
	}()

	if err == nil {
		bytes := make([]byte, 1000)
		count, _ := secret.Read(bytes)
		err = json.Unmarshal(bytes[:count], cred)
		if err != nil {
			return nil,err
		}
		return cred,nil
	}

	account, err := oauth2l.StartAuth(sysinf)
	if err != nil {
		return nil, err
	}

	cred.Username = account.Username
	cred.Password = account.Password
	bytes,_ := json.MarshalIndent(cred," "," ")
	if _, err = secret.Write(bytes); err != nil {
		fmt.Printf("%s\n", err.Error())
	}

	return cred,nil
}


func SetupWorkerAccount(URL string,
					    data Data,
						proxy Cred) (
						cred *Cred, 
						err error) {

	b,_ := json.Marshal(data)
	req,err := http.NewRequest("POST",URL,bytes.NewBuffer(b))
	if err != nil {
		return nil,err
	}

	req.Header.Set("username",proxy.Username)
	req.Header.Set("password",proxy.Password)
	req.Header.Set("Authorization",fmt.Sprintf("Bearer %s","eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZS1kZW1vIiwicm9sZSI6ImFub24iLCJleHAiOjE5ODM4MTI5OTZ9.CRXP1A7WOeoJeXxjNni43kdQwgnWNReilDMblYTn_I0"))

	resp,err := http.DefaultClient.Do(req)
	if err != nil {
		return nil,err
	}


	body := make([]byte, 10000)
	size,_ := resp.Body.Read(body)
	if resp.StatusCode != 200 {
		body_str := string(body[:size])
		return nil,fmt.Errorf("response code %d: %s",resp.StatusCode,body_str)
	}

	if err := json.Unmarshal(body[:size],&proxy);err != nil {
		return nil,err
	}

	return &proxy,nil
}