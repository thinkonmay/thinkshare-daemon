package media

import (
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"github.com/thinkonmay/thinkshare-daemon/persistent/gRPC/packet"
	"github.com/thinkonmay/thinkshare-daemon/utils/pipeline"
)

/*
#include <gst/gst.h>
#include <stdio.h>



typedef struct _Soundcard {
    char device_id[500];
    char name[500];
    char api[50];

    gboolean isdefault;
    gboolean loopback;

    int active;
}Soundcard;

typedef struct _Mic {
    char device_id[500];
    char name[500];
    char api[50];

    gboolean isdefault;
    gboolean loopback;

    int active;
}Mic;

typedef struct _MediaDevice
{
    Soundcard soundcards[10];
    Mic microphones[10];
}MediaDevice;

static void
device_foreach(GstDevice* device,
               gpointer data)
{
    MediaDevice* source = (MediaDevice*) data;
    gchar* klass = gst_device_get_device_class(device);

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
        if(!g_strcmp0(api,"wasapi")) {
            int i = 0;
            while (source->microphones[i].active) { i++; }
            Mic* mic = &source->microphones[i];

            gchar* strid = (gchar*)gst_structure_get_string(device_structure,"device.strid");
            memcpy(mic->device_id,strid,strlen(strid));
            memcpy(mic->api,api,strlen(api));
            gchar* name = gst_device_get_display_name(device);
            memcpy(mic->name,name,strlen(name));

            mic->active = TRUE;
        } else {
            g_object_unref(device);
            return;
        }
    }
    g_object_unref(device);
}



MediaDevice*
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

    return &dev;
}

#cgo pkg-config: gstreamer-1.0
*/
import "C"

type DeviceQuery unsafe.Pointer



var (
    virtual_displays []*os.Process = []*os.Process{}
)


func execute(dir string,name string, args ...string) {
    cmd := exec.Command(name,args...)
    cmd.Dir = dir
    b,err := cmd.Output()
    if err != nil {
        fmt.Println(dir + " failed : " + err.Error())
    } else {
        fmt.Println(dir + " sucess : " + string(b))
    }
}

func ActivateVirtualDriver() {
    fmt.Println("Activating virtual driver")
    execute("./audio",         "./VBCABLE_Setup_x64.exe","-i","-h")
    execute("./microphone",    "./VBCABLE_Setup_x64.exe","-i","-h")
    // execute("./gamepad",       "./nefconc.exe","--install-driver","--inf-path","ViGEmBus.inf")
    execute("./display",       "./CertMgr.exe","/add","IddSampleDriver.cer","/s","/r","localMachine","root")
    execute("./display",       "./nefconc.exe","--install-driver","--inf-path","IddSampleDriver.inf")
}

func DeactivateVirtualDriver() {
    fmt.Println("Deactivating virtual driver")
    execute("./display",       "./nefconc.exe","--uninstall-driver","--inf-path","IddSampleDriver.inf")
    execute("./audio",         "./VBCABLE_Setup_x64.exe","-u","-h")
    execute("./microphone",    "./VBCABLE_Setup_x64.exe","-u","-h")
    // execute("./gamepad",       "./nefconc.exe","--uninstall-driver","--inf-path","ViGEmBus.inf")
    for _, p := range virtual_displays {
        p.Kill()
    }
}


func StartVirtualDisplay() *os.Process {
    width  := 1920
    height := 1200

    cmd := exec.Command("./IddSampleApp.exe")
    cmd.Dir = "./display"
    cmd.Env = []string{
        fmt.Sprintf("DISPLAY_WIDTH=%d",width),
        fmt.Sprintf("DISPLAY_HEIGHT=%d",height),
    }
    cmd.Start()
    virtual_displays = append(virtual_displays, cmd.Process)
    return cmd.Process
}

func GetDevice() *packet.MediaDevice {
	result := &packet.MediaDevice{ }
	query := C.query_media_device()

    for _,mic := range query.microphones {
        if mic.active == 0 {
            continue
        }
        m := &packet.Microphone{
			Name:       C.GoString(&mic.name[0]),
			DeviceID:   C.GoString(&mic.device_id[0]),
			Api:        C.GoString(&mic.api[0]) + "-in",
        }

        var err error
        if m.Pipeline,err = pipeline.MicPipeline(m);err != nil {
            continue
        }
        result.Microphone = m
    }

    for _,sound := range query.soundcards {
        if sound.active == 0 {
            continue
        }
        m := &packet.Soundcard{
			Name:       C.GoString(&sound.name[0]),
			DeviceID:   C.GoString(&sound.device_id[0]),
			Api:        C.GoString(&sound.api[0]) + "-out",
        }


        var err error
        if m.Pipeline,err = pipeline.AudioPipeline(m);err != nil {
            continue
        }
    }

	return result
}