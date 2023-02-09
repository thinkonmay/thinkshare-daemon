package ice

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/pion/webrtc/v3"
	"github.com/thinkonmay/thinkshare-daemon/log"
)

func TestFilter(t *testing.T) {
	go func() {
		for {
			_log := log.TakeLog()
			fmt.Println(_log)
		}
	}()

	rtc := webrtc.Configuration{ICEServers: []webrtc.ICEServer{{
		URLs: []string{ "stun:stun.l.google.com:19302" },
	}, {
		URLs:           []string{"turn:workstation.thinkmay.net:3478"},
		Username:       "oneplay",
		Credential:     "oneplay",
		CredentialType: webrtc.ICECredentialTypePassword,
	}, {
		URLs:           []string{"turn:52.66.204.210:3478"},
		Username:       "oneplay",
		Credential:     "oneplay",
		CredentialType: webrtc.ICECredentialTypePassword,
	}},
	}

	str     := EncodeWebRTCConfig(rtc)
	result2 := DecodeWebRTCConfig(str)

	str2, _ := json.MarshalIndent(result2, " ", " ")
	fmt.Printf("%s\n", str2)
}
