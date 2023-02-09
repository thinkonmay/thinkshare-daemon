package signaling

import (
	"encoding/base64"
	"encoding/json"
)

type Signaling struct {
	Wsurl    string `json:"wsurl"`
	Grpcport int    `json:"grpcport"`
	Grpcip   string `json:"grpcip"`
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