package session 

import (
	"encoding/base64"
	"encoding/json"
	"github.com/pion/webrtc/v3"
)






func EncodeWebRTCConfig(config webrtc.Configuration) string {
	bytes, _ := json.Marshal(config)
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func DecodeWebRTCConfig(data string) webrtc.Configuration {
	bytes, _ := base64.RawURLEncoding.DecodeString(data)

	result := webrtc.Configuration{}
	json.Unmarshal(bytes, &result)
	return result
}
