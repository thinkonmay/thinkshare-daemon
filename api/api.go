package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/thinkonmay/thinkshare-daemon/api/session"
	"github.com/thinkonmay/thinkshare-daemon/utils/system"
)



func GetServerToken(URL string, sysinf *system.SysInfo) (token string, err error) {
	infor, err := json.Marshal(sysinf)
	if err != nil {
		fmt.Printf("unable to marshal sysinfor :%s\n", err.Error())
		return
	}

	req, err := http.NewRequest("POST", URL, bytes.NewBuffer(infor))
	req.Header.Set("Content-Type", "application/json")
	if err != nil {
		fmt.Printf("unable to request :%s\n", err.Error())
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("unable to request :%s\n", err.Error())
		return
	} else if resp.StatusCode != 200 {
		out := make([]byte, 10000)
		size, _ := resp.Body.Read(out)
		return "", fmt.Errorf("unable to request :%s", string(out[:size]))
	}

	out := make([]byte, 10000)
	size, err := resp.Body.Read(out)
	if err != nil {
		fmt.Printf("unable to request :%s\n", err.Error())
		return
	}
	return string(out[:size]), nil
}



func GetSessionToken(URL string, token string) (out string, err error) {
	req, err := http.NewRequest("GET", URL, bytes.NewBuffer([]byte("")))
	req.Header.Add("Authorization", fmt.Sprintf("Brearer %s", token))
	if err != nil {
		err = fmt.Errorf("unable to request %s", err.Error())
		return
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		err = fmt.Errorf("%s", err.Error())
		return
	} else if res.StatusCode != 200 {
		err = fmt.Errorf("server response code %d", res.StatusCode)
		return
	} else {
		buff := make([]byte, 10000)
		size, _ := res.Body.Read(buff)
		return string(buff[:size]), nil
	}
}



func GetSessionInfor(URL string, ssToken string) (*session.Session, error) {
	req, err := http.NewRequest("GET",
		fmt.Sprintf("%s?token=%s", URL, ssToken),
		bytes.NewBuffer([]byte("")))

	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	buff := make([]byte, 10000)
	size, err := res.Body.Read(buff)
	if err != nil {
		return nil, err
	}

	result := &session.Session{ Token: ssToken, }
	result.GetSessionInforHash(buff[:size])
	return result, nil
}


