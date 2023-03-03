package credential

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/pigeatgarlic/oauth2l"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)

type Cred struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
type SupabaseConfig struct {
	URL string `json:"url"`
	KEY string `json:"key"`
}



type WorkerProfile struct {
	ID 		  int			`json:"id"`	
	AccountID string		`json:"account_id"`

	Active    bool 			`json:"active"`
	
	InsertAt   string		`json:"inserted_at"`
	LastUpdate string		`json:"last_update"`

	Info	  system.SysInfo	`json:"metadata"`
}


type SupabaseDatabase struct {
	account_id string
	conf SupabaseConfig
	cred Cred

	profile WorkerProfile	
}





func SetupSupabaseDb(sysinf *system.SysInfo) (db *SupabaseDatabase,err error){
	db = &SupabaseDatabase{}



	result, err := os.ReadFile("./supabase.json")
	if err != nil {
		return
	}

	json.Unmarshal(result, &db.conf)
	if err != nil {
		return
	}

	var dofetch bool
	secret, err := os.OpenFile("./cache.secret.json", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		dofetch = true
	} else {
		bytes := make([]byte, 1000)
		count, _ := secret.Read(bytes)
		err = json.Unmarshal(bytes[:count], &db.cred)
		dofetch = (err != nil)
	}

	if dofetch {
		account, err := oauth2l.StartAuth(sysinf)
		if err != nil {
			return nil, err
		}

		db.cred.Username = account.Username
		db.cred.Password = account.Password

		bytes,err := json.Marshal(db.cred)
		if err != nil {
			fmt.Printf("%s", err.Error())
		}
		if  _, err = secret.Write(bytes); err != nil {
			fmt.Printf("%s", err.Error())
		}
		if err := secret.Close(); err != nil {
			fmt.Printf("%s", err.Error())
		}
	}


	return
}