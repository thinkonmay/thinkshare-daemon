package iceservers

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-ping/ping"
	"github.com/pion/webrtc/v3"
)

func filterWebRTCConfig(config webrtc.Configuration) webrtc.Configuration {
	result := webrtc.Configuration{}

	total_turn, count := 0, 0
	pingResults := map[string]time.Duration{}
	for _, ice := range config.ICEServers {
		splits := strings.Split(ice.URLs[0], ":")
		if splits[0] == "turn" {
			total_turn++

			go func(ice_ webrtc.ICEServer) {
				defer func() {
					count++
				}()

				domain := splits[1]
				pinger, err := ping.NewPinger(domain)
				pinger.SetPrivileged(true)
				if err != nil {
					return
				}
				pinger.Count = 3
				pinger.Timeout = time.Second
				err = pinger.Run() // Blocks until finished.
				if err != nil {
					return
				}

				stats := pinger.Statistics() // get send/receive/duplicate/rtt stats
				if stats.AvgRtt != 0 {
					pingResults[ice_.URLs[0]] = stats.AvgRtt
				}
			}(ice)
		}
	}

	for {
		time.Sleep(100 * time.Millisecond)
		if count == total_turn {
			break
		}
	}

	minUrl, minDuration := "", 100*time.Second
	for url, result := range pingResults {
		if result < minDuration {
			minDuration = result
			minUrl = url
		}
	}

	for _, ice := range config.ICEServers {
		if ice.URLs[0] == minUrl {
			result.ICEServers = append(result.ICEServers, ice)

			result.ICEServers = append(result.ICEServers, webrtc.ICEServer{
				URLs: []string{fmt.Sprintf("stun:%s:3478", strings.Split(ice.URLs[0], ":")[1])},
			})
		}
	}

	return result
}







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
