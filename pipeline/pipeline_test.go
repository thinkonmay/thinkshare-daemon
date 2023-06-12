package pipeline

import (
	"fmt"
	"testing"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/pipeline/device"
)

func TestTest(t *testing.T) {
	dev := device.GetDevice()
	result, _, err := GstTestVideo(int(dev.Monitors[0].MonitorHandle),"")
	if err != nil {
		panic(err)
	}

	fmt.Printf("test %s\n", result)

	souncard := packet.Soundcard{}
	for _, card := range dev.Soundcards {
		if card.Name == "Default Audio Render Device" {
			souncard = *card
		}
	}

	result, err = GstTestAudio(souncard.Api, souncard.DeviceID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", result)
}

func TestSync(t *testing.T) {
	// dev := device.GetDevice()

	// video := &VideoPipeline { }
	// video.SyncPipeline(dev.Monitors[0])

	// soundcard := &packet.Soundcard{}
	// for _,sc := range dev.Soundcards {
	// 	if sc.Name == "Default Audio Render Device"  {
	// 		soundcard = sc
	// 	}
	// }
	// audio := &AudioPipeline { }
	// audio.SyncPipeline(soundcard)

	// fmt.Printf("%v",audio)

}
