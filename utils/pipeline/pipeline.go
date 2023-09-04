package pipeline

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/thinkonmay/thinkshare-daemon/utils/path"
)

const (
	VideoClockRate = 90000
	AudioClockRate = 48000

	defaultAudioBitrate = 256000
	defaultVideoBitrate = 6000
)

func AudioPipeline(card *packet.Soundcard) (*packet.Pipeline, error) {
	result, err := GstTestAudio(card.Api, card.Name ,card.DeviceID)
	if err != nil {
		return nil, err
	}

	pipeline := &packet.Pipeline{}
	pipeline.PipelineString = result
	pipeline.Plugin = card.Api

	bytes, _ := json.Marshal(pipeline.PipelineString)
	pipeline.PipelineHash = base64.StdEncoding.EncodeToString(bytes)
	return pipeline, nil
}

func VideoPipeline(monitor *packet.Monitor) (*packet.Pipeline, error) {
	result, plugin, err := GstTestVideo(int(monitor.MonitorHandle),monitor.Adapter)
	if err != nil {
		return nil, err
	}

	pipeline := &packet.Pipeline{}
	pipeline.PipelineString = result
	pipeline.Plugin = plugin

	// possible memory leak here, severity HIGH, avoid calling this if possible
	bytes, _ := json.Marshal(pipeline.PipelineString)
	pipeline.PipelineHash = base64.StdEncoding.EncodeToString(bytes)
	return pipeline, nil
}

func MicPipeline(card *packet.Microphone) (*packet.Pipeline, error) {
	result, err := GstTestAudioIn(card.Api, card.Name ,card.DeviceID)
	if err != nil {
		return nil, err
	}

	pipeline := &packet.Pipeline{}
	pipeline.PipelineString = result
	pipeline.Plugin = card.Api

	bytes, _ := json.Marshal(pipeline.PipelineString)
	pipeline.PipelineHash = base64.StdEncoding.EncodeToString(bytes)
	return pipeline, nil
}

func findTestCmd(plugin string, handle int, DeviceID string) *exec.Cmd {
	path, err := path.FindProcessPath("", "gst-launch-1.0.exe")
	if err != nil {
		panic(err)
	}
	switch plugin {
	case "nvcodec":
		return exec.Command(path, 
			"tmd3d11screencapturesrc", "blocksize=8192", "do-timestamp=true", "show-cursor=false","name=capturer",
			fmt.Sprintf("monitor-handle=%d", handle), "!", "capsfilter", "name=framerateFilter", "!", 
			fmt.Sprintf("video/x-raw(memory:D3D11Memory),framerate=55/1,clock-rate=%d", VideoClockRate), "!", 
			"queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"tmd3d11convert", "!", 
			"queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"tmnvd3d11h264enc", "preset=5","rate-control=2","strict-gop=true","repeat-sequence-header=true","zero-reorder-delay=true",
			fmt.Sprintf("bitrate=%d", defaultVideoBitrate), "name=encoder", "!", 
			"video/x-h264,stream-format=(string)byte-stream,profile=(string)main", "!", 
			"queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"appsink", "name=appsink")
	case "quicksync":
		return exec.Command(path, 
			"tmd3d11screencapturesrc", "blocksize=8192", "do-timestamp=true", "show-cursor=false","name=capturer",
			fmt.Sprintf("monitor-handle=%d", handle), "!", "capsfilter", "name=framerateFilter", "!", 
			fmt.Sprintf("video/x-raw(memory:D3D11Memory),framerate=55/1,clock-rate=%d", VideoClockRate), "!", 
			"queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"tmd3d11convert", "!", 
			"queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"tmqsvh264enc", 
			fmt.Sprintf("bitrate=%d", defaultVideoBitrate), "name=encoder", "!", 
			"video/x-h264,stream-format=(string)byte-stream,profile=(string)main", "!", 
			"queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"appsink", "name=appsink")
	case "amf":
		return exec.Command(path, 
			"tmd3d11screencapturesrc", "blocksize=8192", "do-timestamp=true", "show-cursor=false","name=capturer",
			fmt.Sprintf("monitor-handle=%d", handle), "!", "capsfilter", "name=framerateFilter", "!", 
			fmt.Sprintf("video/x-raw(memory:D3D11Memory),framerate=55/1,clock-rate=%d", VideoClockRate), "!", 
			"queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"tmd3d11convert", "!", 
			"queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"tmamfh264enc", 
			fmt.Sprintf("bitrate=%d", defaultVideoBitrate),  "name=encoder", "!", 
			"video/x-h264,stream-format=(string)byte-stream,profile=(string)main", "!", 
			"queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"h264parse", "config-interval=-1", "!", 
			"queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"appsink", "name=appsink")
	case "wasapi-out":
		return exec.Command(path, 
			"wasapisrc", "name=source", 
			fmt.Sprintf("device=%s", formatAudioDeviceID(DeviceID)), "!", 
			"audioresample", "!", 
			fmt.Sprintf("audio/x-raw,rate=%d", AudioClockRate), "!", 
			"audioconvert", "!", 
			"opusenc", "name=encoder", "!", 
			"appsink", "name=appsink")
	case "wasapi-in":
		return exec.Command(path, 
			"appsrc","format=time","is-live=true","do-timestamp=true","name=appsrc","!",
			"application/x-rtp,payload=96,encoding-name=OPUS,clock-rate=48000","!",
			"rtpopusdepay","!",
			"opusdec","!",
			"audioconvert","!",
			"audioresample","!",
			"queue","!",
			"wasapisink",
			fmt.Sprintf("device=%s",formatAudioDeviceID(DeviceID)))
	default:
		return nil
	}
}

func formatAudioDeviceID(in string) string {
	modified := make([]byte, 0)
	byts := []byte(in)

	for index, byt := range byts {
		if byts[index] == []byte("{")[0] ||
			byts[index] == []byte("?")[0] ||
			byts[index] == []byte("#")[0] ||
			byts[index] == []byte("}")[0] {
			modified = append(modified, []byte("\\")[0])
		}
		modified = append(modified, byt)
	}

	ret := []byte("\"")
	ret = append(ret, modified...)
	ret = append(ret, []byte("\"")...)
	return string(ret)
}

func GstTestAudio(API string, adapter string, DeviceID string) (string, error) {
	testcase := findTestCmd(API, 0, DeviceID)
	pipeline, err := gstTestGeneric(API,adapter, testcase)
	if err != nil {
		return "", err
	}

	return pipeline, nil
}
func GstTestAudioIn(API string, adapter string, DeviceID string) (string, error) {
	testcase := findTestCmd(API, 0, DeviceID)
	pipeline, err := gstTestGeneric(API,adapter, testcase)
	if err != nil {
		return "", err
	}

	return pipeline, nil
}

func GstTestVideo(MonitorHandle int,
				  adapter string,
				  ) (pipeline string, 
					plugin string, 
					err error) {
	video_plugins := []string{"nvcodec", "amf", "quicksync" }

	for _, _plugin := range video_plugins {
		testcase := findTestCmd(_plugin, MonitorHandle, "")
		pipeline, err := gstTestGeneric(_plugin,adapter, testcase)
		if err != nil {
			log.PushLog("test failted %s", err.Error())
			continue
		}

		return pipeline, _plugin, nil
	}

	return "", "", fmt.Errorf("no suitable pipeline found")
}

func gstTestGeneric(plugin string,
					adapter string, 
					testcase *exec.Cmd,
					) (pipeline string,err error) {
	defer func() {
		if err != nil {
			log.PushLog("pipeline %s test failed", plugin)
		} else {
			log.PushLog("pipeline %s test success", plugin)
		}
	}()
	if testcase == nil {
		return "", fmt.Errorf("nil test case")
	}

	intel 		:= strings.Contains(strings.ToLower(adapter),"intel")
	nvidia 		:= strings.Contains(strings.ToLower(adapter),"geforce")  || strings.Contains(strings.ToLower(adapter),"nvidia")
	amd 		:= strings.Contains(strings.ToLower(adapter),"radeon")   || strings.Contains(strings.ToLower(adapter),"amd")
	microphone 	:= strings.Contains(strings.ToLower(adapter),"microphone")
	headset 	:= strings.Contains(strings.ToLower(adapter),"headset")
	vbcableout  := strings.Contains(strings.ToLower(adapter),"cable output") 
	vbcablein   := strings.Contains(strings.ToLower(adapter),"cable-a input") 
	wasapi      := strings.Contains(strings.ToLower(plugin),"wasapi") 

	// quick table
	if plugin == "nvcodec" && nvidia {
		return strings.Join(testcase.Args[1:], " "), nil
	} else if plugin == "nvcodec" && (intel || amd) {
		return "", fmt.Errorf("test program failed")
	} else if plugin == "amf" &&  amd {
		return strings.Join(testcase.Args[1:], " "), nil
	} else if plugin == "amf" && (intel || nvidia) {
		return "", fmt.Errorf("test program failed")
	} else if plugin == "quicksync" && intel {
		return strings.Join(testcase.Args[1:], " "), nil
	} else if plugin == "quicksync" && (amd || nvidia) {
		return "", fmt.Errorf("test program failed")
	} else if plugin == "wasapi-out" && vbcableout {
		return strings.Join(testcase.Args[1:], " "), nil
	} else if plugin == "wasapi-out" && !vbcableout {
		return "", fmt.Errorf("test program failed")
	} else if plugin == "wasapi-in" && vbcablein {
		return strings.Join(testcase.Args[1:], " "), nil
	} else if plugin == "wasapi-in" && !vbcablein {
		return "", fmt.Errorf("test program failed")
	} else if wasapi && (microphone || headset) {
		return "", fmt.Errorf("test program failed")
	}

	done := make(chan bool, 2)

	go func() {
		testcase.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		err = testcase.Run()
		done <- false
	}()
	go func() {
		time.Sleep(10 * time.Second)
		done <- true
	}()

	success := <-done

	if success {
		testcase.Process.Kill()
		return strings.Join(testcase.Args[1:], " "), nil
	} else if err != nil {
		return "", fmt.Errorf("test program failed, err: %s", err.Error())
	} else {
		return "", fmt.Errorf("test program failed")
	}
}
