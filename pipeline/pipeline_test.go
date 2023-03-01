package gsttest

import (
	"fmt"
	"testing"

	"github.com/thinkonmay/thinkshare-daemon/pipeline/device"
)

func TestTest(t *testing.T) {
	dev := device.GetDevice()
	result,err := GstTestVideo(&dev.Monitors[0])
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

	result,err = GstTestAudio(&souncard)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", result)
}
