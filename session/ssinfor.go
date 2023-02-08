package session

import (
	"encoding/base64"
	"encoding/json"

	"github.com/pion/webrtc/v3"
	iceservers "github.com/thinkonmay/thinkshare-daemon/ice-servers"
)

type Signaling struct {
	Wsurl    string `json:"wsurl"`
	Grpcport int    `json:"grpcport"`
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

func EncodeSignalingConfig(config Signaling) string {
	bytes, _ := json.Marshal(config)
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func DecodeSignalingConfig(data string) Signaling {
	bytes, _ := base64.RawURLEncoding.DecodeString(data)
	result := Signaling{}
	json.Unmarshal(bytes, &result)
	return result
}

func GetSessionInforHash(in []byte) (webrtcHash string, signaling string) {
	ssinfor := SessionInfor{}
	json.Unmarshal(in, &ssinfor)
	webrtc_config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{
			URLs: ssinfor.STUNs,
		}}}

	for _, i := range ssinfor.TURNs {
		webrtc_config.ICEServers = append(webrtc_config.ICEServers, webrtc.ICEServer{
			URLs:           []string{i.URL},
			Username:       i.Username,
			Credential:     i.Credential,
			CredentialType: webrtc.ICECredentialTypePassword,
		})
	}

	webrtcHash = iceservers.FilterAndEncodeWebRTCConfig(webrtc_config)
	signaling = EncodeSignalingConfig(ssinfor.Signaling)

	return
}
