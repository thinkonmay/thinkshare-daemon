package pipeline 

import (
	"fmt"
	"testing"

	"github.com/thinkonmay/thinkshare-daemon/pipeline/device"
)

func TestTest(t *testing.T) {
	dev := device.GetDevice()
	result,_,err := GstTestVideo(dev.Monitors[0].MonitorHandle)
	if err != nil {
		panic(err)
	}

	fmt.Printf("test %s\n", result)

	souncard := device.Soundcard{}
	for _, card := range dev.Soundcards {
		if card.Name == "Default Audio Render Device" {
			souncard = card
		}
	}

	result,err = GstTestAudio(souncard.Api,souncard.DeviceID)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", result)
}



func TestSync(t *testing.T) {
	dev := device.GetDevice()


	video := &VideoPipeline {
		PipelineString: map[int]string{},
		Plugin: map[int]string{},
	}

	video.SyncPipeline(dev)

	audio := &AudioPipeline {
	}

	audio.SyncPipeline(dev)

	fmt.Printf("%v\n%v",video,audio)

}
