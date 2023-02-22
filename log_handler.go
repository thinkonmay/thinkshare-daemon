package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/thinkonmay/thinkshare-daemon/log"
	"github.com/thinkonmay/thinkshare-daemon/service"
)

func DefaultLogHandler(daemon *service.Daemon, enableLogfile bool, enableWebscoketLog bool) {
	log_file,err := os.OpenFile("./log.txt", os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.PushLog("error open log file to write: %s",err.Error())
		log_file = nil
		err = nil
	} else {
		log_file.Truncate(0);
	}


	var wsclient *websocket.Conn = nil
	go func ()  {
		var wserr error
		for {
			time.Sleep(5 * time.Second)
			if daemon.ServerToken == "none" || wsclient != nil {
				continue;
			}



			wsclient, _, wserr = websocket.DefaultDialer.Dial(daemon.LogURL,http.Header{
				"Authorization": []string{fmt.Sprintf("Bearer %s",daemon.ServerToken)},
			})

			if wserr != nil {
				log.PushLog("error setup log websocket : %s",wserr.Error())
				wsclient = nil
			} else {
				wsclient.SetCloseHandler(func(code int, text string) error {
					wsclient = nil
					return nil
				})
			}
		}
	}()




	go func ()  {
		for {
			out := log.TakeLog()
			
			if wsclient != nil {
				err := wsclient.WriteMessage(websocket.TextMessage,[]byte(out));
				if err != nil {
					wsclient = nil
				}
			}

			if log_file != nil {
				log_file.Write([]byte(fmt.Sprintf("%s\n",out)))
			}
		}
	}()
}