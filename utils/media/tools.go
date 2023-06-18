package media

import (
	"time"
	"unsafe"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
)


/*
#include <gst/gst.h>
#include <stdio.h>




int           
string_get_length(void* string)
{
    return strlen((char*)string);
}

typedef struct _Soundcard {
    char device_id[500];
    char name[500];
    char api[50];

    gboolean isdefault;
    gboolean loopback;

    int active;
}Soundcard;

typedef struct _Monitor {
    guint64 monitor_handle;
    int primary;

    char device_name[100];
    char adapter[100];
    char monitor_name[100];

    int width, height;

    int active;
}Monitor;

typedef struct _MediaDevice
{
    Monitor monitors[10];
    Soundcard soundcards[10];
    Soundcard micro[10];
}MediaDevice;

static void
device_foreach(GstDevice* device, 
               gpointer data)
{
    MediaDevice* source = (MediaDevice*) data;
    gchar* klass = gst_device_get_device_class(device);

    // handle monitor
    if(!g_strcmp0(klass,"Source/Monitor")) {
        GstStructure* device_structure = gst_device_get_properties(device);
        gchar* api = (gchar*)gst_structure_get_string(device_structure,"device.api");
        if(g_strcmp0(api,"d3d11")) {
            g_object_unref(device);
            return;
        }

        int i = 0;
        while (source->monitors[i].active) { i++; }
        Monitor* monitor = &source->monitors[i];

        gchar* name = gst_device_get_display_name(device);
        memcpy(monitor->monitor_name,name,strlen(name));
        monitor->active = TRUE;

        gchar* adapter = (gchar*)gst_structure_get_string(device_structure,"device.adapter.description");
        memcpy(monitor->adapter,adapter,strlen(adapter));

        gchar* device_name = (gchar*)gst_structure_get_string(device_structure,"device.name");
        memcpy(monitor->device_name,device_name,strlen(device_name));

        int top, left, right, bottom = 0;
        gst_structure_get_int(device_structure,"display.coordinates.right",&right);
        gst_structure_get_int(device_structure,"display.coordinates.top",&top);
        gst_structure_get_int(device_structure,"display.coordinates.left",&left);
        gst_structure_get_int(device_structure,"display.coordinates.bottom",&bottom);

        monitor->width =  right - left;
        monitor->height = bottom - top;
        
         
        gst_structure_get_uint64(device_structure,"device.hmonitor",&monitor->monitor_handle);
        gst_structure_get_boolean(device_structure,"device.primary",&monitor->primary);
    }
    
    // handle audio
    if(!g_strcmp0(klass,"Audio/Source")) {
        GstStructure* device_structure = gst_device_get_properties(device);
        gchar* api = (gchar*)gst_structure_get_string(device_structure,"device.api");
        if(!g_strcmp0(api,"wasapi")) {
            int i = 0;
            while (source->soundcards[i].active) { i++; }
            Soundcard* soundcard = &source->soundcards[i];

            memcpy(soundcard->api,api,strlen(api));

            gchar* name = gst_device_get_display_name(device);
            memcpy(soundcard->name,name,strlen(name));
            soundcard->active = TRUE;

            gchar* device_name = (gchar*)gst_structure_get_string(device_structure,"wasapi.device.description");
            memcpy(soundcard->name,device_name,strlen(device_name));

            gchar* strid = (gchar*)gst_structure_get_string(device_structure,"device.strid");
            memcpy(soundcard->device_id,strid,strlen(strid));
        } else if (!g_strcmp0(api,"wasapi2")) {
            int i = 0;
            while (source->soundcards[i].active) { i++; }
            Soundcard* soundcard = &source->soundcards[i];

            memcpy(soundcard->api,api,strlen(api));
            gst_structure_get_boolean(device_structure,"device.default",&soundcard->isdefault);
            gst_structure_get_boolean(device_structure,"wasapi2.device.loopback",&soundcard->loopback);

            gchar* name = gst_device_get_display_name(device);
            memcpy(soundcard->name,name,strlen(name));
            soundcard->active = TRUE;


            gchar* strid = (gchar*)gst_structure_get_string(device_structure,"device.id");
            memcpy(soundcard->device_id,strid,strlen(strid));
        } else {
            g_object_unref(device);
            return;
        }
    }

    // handle audio
    if(!g_strcmp0(klass,"Audio/Sink")) {
        GstStructure* device_structure = gst_device_get_properties(device);
        gchar* api = (gchar*)gst_structure_get_string(device_structure,"device.api");
        if(g_strcmp0(api,"wasapi2") && g_strcmp0(api,"wasapi")) {
            g_object_unref(device);
            return;
        }
    }
    g_object_unref(device);
}



void*
query_media_device()
{
    static MediaDevice dev;
    memset(&dev,0,sizeof(MediaDevice));

    gst_init(NULL, NULL);
    GstDeviceMonitor* monitor = gst_device_monitor_new();
    if(!gst_device_monitor_start(monitor)) {
        return (void*)"fail to start device monitor";
    }

    GList* device_list = gst_device_monitor_get_devices(monitor);
    g_list_foreach(device_list,(GFunc)device_foreach,&dev);

    return (void*)&dev;
}

void*
get_monitor_device_name(void* data,
                 int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->monitors[pos].device_name;
}

void*
get_monitor_name(void* data,
                    int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->monitors[pos].monitor_name;
}

int
get_monitor_width(void* data,
                   int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return (int)source->monitors[pos].width;
}
int
get_monitor_height(void* data,
                   int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return (int)source->monitors[pos].height;
}
int
get_monitor_handle(void* data,
                   int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return (int)source->monitors[pos].monitor_handle;
}

void*
get_monitor_adapter(void* data,
                    int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->monitors[pos].adapter;
}
int   
monitor_is_primary(void* data, 
                  int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->monitors[pos].primary;
}


int   
monitor_is_active(void* data, 
                  int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->monitors[pos].active;
}

int   
soundcard_is_active(void* data, 
                  int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->soundcards[pos].active;
}
int   
soundcard_is_default(void* data, 
                  int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->soundcards[pos].isdefault;
}
int   
soundcard_is_loopback(void* data, 
                  int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->soundcards[pos].loopback;
}

void*
get_soundcard_api(void* data, 
                  int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->soundcards[pos].api;
}
void*
get_soundcard_name(void* data, 
                  int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->soundcards[pos].name;
}

void*
get_soundcard_device_id(void* data, 
                  int pos)
{
    MediaDevice* source = (MediaDevice*) data;
    return source->soundcards[pos].device_id;
}


#cgo pkg-config: gstreamer-1.0 
*/
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
