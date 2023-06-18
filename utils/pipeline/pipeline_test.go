package pipeline

import (
	"fmt"
	"testing"

	"github.com/thinkonmay/thinkshare-daemon/utils/media"
)

func TestTest(t *testing.T) {
	dev := media.GetDevice()
	for _, m := range dev.Monitors {
		result, _, err := GstTestVideo(int(m.MonitorHandle),m.Adapter)
		if err != nil {
			panic(err)
		}
		fmt.Printf("%s\n%s\n%s\n",m.DeviceName,m.Adapter, result)
	}

	for _, card := range dev.Soundcards {
		souncard := *card
		result, err := GstTestAudio(souncard.Api, souncard.Name, souncard.DeviceID)
		if err != nil {
			continue
		}
		fmt.Printf("%s\n%s\n", card.Name, result)
	}
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
