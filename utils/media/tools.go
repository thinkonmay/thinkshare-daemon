package media

import (
	"time"
	"unsafe"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)

// #cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0
// #cgo LDFLAGS: ${SRCDIR}/../../cgo/lib/libshared.a
// #include "util.h"
import "C"

type DeviceQuery unsafe.Pointer

func GetDevice() *packet.MediaDevice {
	result := &packet.MediaDevice{
		Timestamp: time.Now().Format(time.RFC3339),
	}
	query := C.query_media_device()

	count_soundcard := C.int(0)
	count_monitor := C.int(0)
	for {
		active := C.monitor_is_active(query, count_monitor)
		if active == 0 {
			break
		}
		mhandle := C.get_monitor_handle(query, count_monitor)
		monitor_name := C.get_monitor_name(query, count_monitor)
		adapter := C.get_monitor_adapter(query, count_monitor)
		device_name := C.get_monitor_device_name(query, count_monitor)
		width := C.get_monitor_width(query, count_monitor)
		height := C.get_monitor_height(query, count_monitor)
		prim := C.monitor_is_primary(query, count_monitor)

		result.Monitors = append(result.Monitors, &packet.Monitor{
			Framerate:     60,
			MonitorHandle: int32(mhandle),
			MonitorName:   string(C.GoBytes(monitor_name, C.string_get_length(monitor_name))),
			Adapter:       string(C.GoBytes(adapter, C.string_get_length(adapter))),
			DeviceName:    string(C.GoBytes(device_name, C.string_get_length(device_name))),
			Width:         int32(width),
			Height:        int32(height),
			IsPrimary:     (prim == 1),
		})
		count_monitor++
	}

	for {
		active := C.soundcard_is_active(query, count_soundcard)
		if active == 0 {
			break
		}
		name := C.get_soundcard_name(query, count_soundcard)
		device_id := C.get_soundcard_device_id(query, count_soundcard)
		api := C.get_soundcard_api(query, count_soundcard)
		loopback := C.soundcard_is_loopback(query, count_soundcard)
		defaul := C.soundcard_is_default(query, count_soundcard)

		result.Soundcards = append(result.Soundcards, &packet.Soundcard{
			Name:       string(C.GoBytes(name, C.string_get_length(name))),
			DeviceID:   string(C.GoBytes(device_id, C.string_get_length(device_id))),
			Api:        string(C.GoBytes(api, C.string_get_length(api))),
			IsDefault:  (defaul == 1),
			IsLoopback: (loopback == 1),
		})
		count_soundcard++
	}

	result.Soundcards = append(result.Soundcards, &packet.Soundcard{
		DeviceID: "none",
		Name:     "Mute audio",
		Api:      "None",

		IsDefault:  false,
		IsLoopback: false,
	})

	return result
}

func ToGoString(str unsafe.Pointer) string {
	if str == nil {
		return ""
	}
	return string(C.GoBytes(str, C.int(C.string_get_length(str))))
}
