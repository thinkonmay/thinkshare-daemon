package pipeline

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/thinkonmay/conductor/protocol/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils"
	"github.com/thinkonmay/thinkshare-daemon/utils/log"
)

const (
	VideoClockRate = 90000
	AudioClockRate = 48000

	defaultAudioBitrate = 256000
	defaultVideoBitrate = 6000
)

type AudioPipeline struct {
	PipelineHash string
	PipelineString string
	Plugin string
}

type VideoPipeline struct {
	PipelineHash string
	PipelineString string
	Plugin string
}

func (pipeline *AudioPipeline) SyncPipeline(card *packet.Soundcard) error {
	result,err := GstTestAudio(card.Api,card.DeviceID)
	if err != nil {
		log.PushLog("unable to find pipeline for soundcard %s",card.DeviceID)
		return err
	}

	pipeline.PipelineString = result;
	pipeline.Plugin = card.Api;

	bytes, _ := json.Marshal(pipeline)
	pipeline.PipelineHash = base64.RawURLEncoding.EncodeToString(bytes)
	return nil
}


func (pipeline *VideoPipeline) SyncPipeline(monitor *packet.Monitor) error {
	result,plugin,err := GstTestVideo(int(monitor.MonitorHandle))
	if err != nil {
		log.PushLog("unable to find pipeline for monitor %s",monitor.MonitorName)
		return err
	}

	pipeline.PipelineString = result;	
	pipeline.Plugin = plugin;	

	// possible memory leak here, severity HIGH, avoid calling this if possible
	bytes, _ := json.Marshal(pipeline)
	pipeline.PipelineHash = base64.RawURLEncoding.EncodeToString(bytes)
	return nil
}






func FindTestCmd(plugin string, handle int, DeviceID string) *exec.Cmd {
	path, err := utils.FindProcessPath("","gst-launch-1.0.exe")
	if err != nil {
		panic(err)
	}
	switch plugin {
	case "media foundation":
		return exec.Command(path, "d3d11screencapturesrc", "blocksize=8192", "do-timestamp=true",
			fmt.Sprintf("monitor-handle=%d", handle),
			"!", "capsfilter", "name=framerateFilter",
			"!", fmt.Sprintf("video/x-raw(memory:D3D11Memory),clock-rate=%d", VideoClockRate),
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"d3d11convert",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"mfh264enc", fmt.Sprintf("bitrate=%d", defaultVideoBitrate), "gop-size=-1", "rc-mode=0", "low-latency=true", "ref=1", "quality-vs-speed=0", "name=encoder",
			"!", "video/x-h264,stream-format=(string)byte-stream,profile=(string)main",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"appsink", "name=appsink")
	case "nvcodec":
		return exec.Command(path, "d3d11screencapturesrc", "blocksize=8192", "do-timestamp=true",
			fmt.Sprintf("monitor-handle=%d", handle),
			"!", "capsfilter", "name=framerateFilter",
			"!", fmt.Sprintf("video/x-raw(memory:D3D11Memory),clock-rate=%d", VideoClockRate),
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"d3d11convert",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"nvd3d11h264enc", fmt.Sprintf("bitrate=%d", defaultVideoBitrate), "gop-size=-1", "preset=5", "rate-control=2", "strict-gop=true", "name=encoder", "repeat-sequence-header=true", "zero-reorder-delay=true",
			"!", "video/x-h264,stream-format=(string)byte-stream,profile=(string)main",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"appsink", "name=appsink")
	case "quicksync":
		return exec.Command(path, "d3d11screencapturesrc", "blocksize=8192", "do-timestamp=true",
			fmt.Sprintf("monitor-handle=%d", handle),
			"!", "capsfilter", "name=framerateFilter",
			"!", fmt.Sprintf("video/x-raw(memory:D3D11Memory),clock-rate=%d", VideoClockRate),
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"d3d11convert",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"qsvh264enc", fmt.Sprintf("bitrate=%d", defaultVideoBitrate), "rate-control=1", "gop-size=-1", "ref-frames=1", "low-latency=true", "target-usage=7", "name=encoder",
			"!", "video/x-h264,stream-format=(string)byte-stream,profile=(string)main",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"appsink", "name=appsink")
	case "amf":
		return exec.Command(path, "d3d11screencapturesrc", "blocksize=8192", "do-timestamp=true",
			fmt.Sprintf("monitor-handle=%d", handle),
			"!", "capsfilter", "name=framerateFilter",
			"!", fmt.Sprintf("video/x-raw(memory:D3D11Memory),clock-rate=%d", VideoClockRate),
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"d3d11convert",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"amfh264enc", fmt.Sprintf("bitrate=%d", defaultVideoBitrate), "rate-control=1", "gop-size=-1", "usage=1", "name=encoder",
			"!", "video/x-h264,stream-format=(string)byte-stream,profile=(string)main",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"h264parse", "config-interval=-1",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"appsink", "name=appsink")
	case "opencodec":
		return exec.Command(path, "d3d11screencapturesrc", "blocksize=8192", "do-timestamp=true",
			fmt.Sprintf("monitor-handle=%d", handle),
			"!", "capsfilter", "name=framerateFilter",
			"!", fmt.Sprintf("video/x-raw(memory:D3D11Memory),clock-rate=%d", VideoClockRate),
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"d3d11convert",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"d3d11download",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"openh264enc", fmt.Sprintf("bitrate=%d", defaultVideoBitrate), "usage-type=1", "rate-control=1", "multi-thread=8", "name=encoder",
			"!", "video/x-h264,stream-format=(string)byte-stream,profile=(string)main",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"appsink", "name=appsink")
	case "wasapi2":
		return exec.Command(path, "wasapi2src", "name=source", "slave-method=1","loopback=true","low-latency=true",
			fmt.Sprintf("device=%s", formatAudioDeviceID(DeviceID)),
			"!", "audio/x-raw",
			"!", "queue", "!",
			"audioresample",
			"!", fmt.Sprintf("audio/x-raw,clock-rate=%d", AudioClockRate),
			"!", "queue", "!",
			"audioconvert",
			"!", "queue", "!",
			"opusenc","audio-type=2051","perfect-timestamp=true","bitrate-type=0","hard-resync=true", fmt.Sprintf("bitrate=%d", defaultAudioBitrate), "name=encoder",
			"!", "queue", "max-size-time=0", "max-size-bytes=0", "max-size-buffers=3", "!",
			"appsink", "name=appsink")
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


func GstTestAudio(API string,DeviceID string) (string,error) {
	testcase := FindTestCmd(API, 0, DeviceID)
	return gstTestGeneric(API, testcase)
}


func GstTestVideo(MonitorHandle int) (pipeline string,plugin string,err error) {
	video_plugins := []string{"nvcodec", "amf", "quicksync", "media foundation", "opencodec"}

	for _, _plugin := range video_plugins {
		fmt.Printf("testing pipeline plugin %s, monitor handle %d\n",_plugin, MonitorHandle)
		testcase := FindTestCmd(_plugin, MonitorHandle, "")
		pipeline,err := gstTestGeneric(_plugin, testcase)
		if err != nil {
			fmt.Printf("test failted %s\n", err.Error())
			continue;
		} 

		log.PushLog("pipeline %s test success",pipeline)
		return pipeline,_plugin,nil;
	}

	return "","",fmt.Errorf("no suitable pipeline found")
}

func gstTestGeneric(plugin string, testcase *exec.Cmd) (string,error) {
	done := make(chan bool,2)

	var err error
	go func() {
		err = testcase.Run()
		done<-false
	}()
	go func() {
		time.Sleep(10 * time.Second)
		done<-true 
	}()

	success:=<-done

	if success {
		testcase.Process.Kill()
		return strings.Join(testcase.Args[1:], " "),nil
	} else if err != nil{
		return "",fmt.Errorf("test program failed, err: %s",err.Error())
	} else {
		return "",fmt.Errorf("test program failed")
	}
}
