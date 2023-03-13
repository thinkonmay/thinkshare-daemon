package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	childprocess "github.com/thinkonmay/thinkshare-daemon/child-process"
	"github.com/thinkonmay/thinkshare-daemon/log"
	"github.com/thinkonmay/thinkshare-daemon/session"
	"github.com/thinkonmay/thinkshare-daemon/system"
)

type Session struct {
	Ids        []childprocess.ProcessID
	Token      string
	WebRTCConf string `json:"webrtcConfig"`
	GrpcConf   string `json:"grpcConfig"`
	Done       bool
}


type SubsystemConnection struct {
	ServerToken string
	SessionToken string


	wsclient *websocket.Conn 
}


func NewSubsystemConnection(registrationURL string) (ret *SubsystemConnection,err error){
	ret = &SubsystemConnection{
		ServerToken: "none",
		SessionToken: "none",
		wsclient: nil,
	}

	if ret.ServerToken,err = GetServerToken(registrationURL); err != nil {
		log.PushLog("unable to get server token :%s\n", err.Error())
		return
	}

	go func ()  {
		var wserr error
		for {
			if ret.ServerToken == "none" || ret.wsclient != nil {
				time.Sleep(1 * time.Second);
				continue;
			}



			ret.wsclient, _, wserr = websocket.DefaultDialer.Dial(registrationURL,http.Header{
				"Authorization": []string{fmt.Sprintf("Bearer %s",ret.ServerToken)},
			})

			if wserr != nil {
				log.PushLog("error setup log websocket : %s",wserr.Error())
				ret.wsclient = nil
			} else {
				ret.wsclient.SetCloseHandler(func(code int, text string) error {
					ret.wsclient = nil
					return nil
				})
			}
			time.Sleep(1 * time.Second);
		}
	}()


	go func ()  {
		for {
			time.Sleep(1 * time.Second);
			if ret.wsclient != nil {
				continue;
			}

			ret.wsclient.WriteMessage(websocket.TextMessage,[]byte("ping"));
			ret.wsclient.WriteMessage(websocket.TextMessage,[]byte("sessionToken"));
		}
	}()
	return
}


func GetServerToken(URL string) (token string, err error) {
	sysinf := system.GetInfor()
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
		err = fmt.Errorf("unable to request %s\n", err.Error())
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



type Signaling struct {
	Wsurl    string `json:"wsurl"`
	Grpcport string `json:"grpcport"`
	Grpcip   string `json:"grpcip"`
}

type TURN struct {
	URL        string `json:"urls"`
	Username   string `json:"username"`
	Credential string `json:"credential"`
}

type SessionInfor struct {
	Signaling Signaling `json:"signaling"`
	TURNs     []TURN    `json:"turns"`
	STUNs     []string  `json:"stuns"`
}

func GetSessionInfor(URL string, ssToken string) (*Session, error) {
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

	result := &Session{
		Token: ssToken,
		Ids:   make([]childprocess.ProcessID, 0),
		Done:  false,
	}

	result.WebRTCConf, result.GrpcConf = session.GetSessionInforHash(buff[:size])
	return result, nil
}


