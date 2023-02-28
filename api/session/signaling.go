package session

import (
	"encoding/base64"
	"encoding/json"
)



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