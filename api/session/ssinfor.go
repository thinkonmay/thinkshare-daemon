package session

import (
	"encoding/json"

	"github.com/pion/webrtc/v3"
)



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
type Session struct {
	Token      string
	WebRTCConf string `json:"webrtcConfig"`
	GrpcConf   string `json:"grpcConfig"`
}


func (session *Session)GetSessionInforHash(in []byte) {
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

	session.WebRTCConf   = EncodeWebRTCConfig(webrtc_config)
	session.GrpcConf     = EncodeSignalingConfig(ssinfor.Signaling)
}
