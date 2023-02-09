package session

import (
	"encoding/json"

	"github.com/pion/webrtc/v3"
	"github.com/thinkonmay/thinkshare-daemon/session/signaling"
	"github.com/thinkonmay/thinkshare-daemon/session/ice"
)



type TURN struct {
	URL        string `json:"urls"`
	Username   string `json:"username"`
	Credential string `json:"credential"`
}

type SessionInfor struct {
	Signaling signaling.Signaling `json:"signaling"`
	TURNs     []TURN    `json:"turns"`
	STUNs     []string  `json:"stuns"`
}

func GetSessionInforHash(in []byte) (webrtcHash string, signalingHash string) {
	ssinfor := SessionInfor{}
	json.Unmarshal(in, &ssinfor)

	webrtc_config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{
			URLs: ssinfor.STUNs,
		}}}

	for _, i := range ssinfor.TURNs {
		webrtc_config.ICEServers = append(webrtc_config.ICEServers, webrtc.ICEServer{
			URLs:       []string{i.URL},
			Username:   i.Username,
			Credential: i.Credential,
		})
	}

	webrtcHash = ice.EncodeWebRTCConfig(webrtc_config)
	signalingHash = signaling.EncodeSignalingConfig(ssinfor.Signaling)
	return
}
