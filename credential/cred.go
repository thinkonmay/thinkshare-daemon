package credential

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pigeatgarlic/oauth2l"
)

type Cred struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type Data struct {
	PublicIP  string `json:"username"`
	PrivateIP string `json:"password"`
}

func SetupProxyAccount(sysinf interface{}) (cred *Cred, err error) {
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
		if err == nil {
			return
		}
	}

	account, err := oauth2l.StartAuth(sysinf)
	if err != nil {
		return nil, err
	}

	cred.Username = account.Username
	cred.Password = account.Password
	bytes, err := json.Marshal(cred)

	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
	if _, err = secret.Write(bytes); err != nil {
		fmt.Printf("%s\n", err.Error())
	}

	return
}
