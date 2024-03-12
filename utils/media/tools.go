package media

/*
#include <Windows.h>
typedef int (*FUNC)	();
typedef int (*FUNC2)	(int width, int height, char* byte, int* size);
typedef int (*FUNC3)	(int index);

static FUNC _init_virtual_display;
static FUNC _deinit_virtual_display;
static FUNC3 _remove_virtual_display;
static FUNC2 _add_virtual_display;

int
initlibrary() {
	void* hModule 	= LoadLibrary(".\\libdisplay.dll");
	_init_virtual_display 	= (FUNC)	GetProcAddress( hModule,"init_virtual_display");
	_deinit_virtual_display = (FUNC)	GetProcAddress( hModule,"deinit_virtual_display");
	_remove_virtual_display	= (FUNC3)	GetProcAddress( hModule,"remove_virtual_display");
	_add_virtual_display 	= (FUNC2)	GetProcAddress( hModule,"add_virtual_display");

    if (_init_virtual_display == 0 ||
        _deinit_virtual_display == 0 ||
        _add_virtual_display == 0 ||
        _remove_virtual_display == 0)
        return 1;


	return 0;
}

int init_virtual_display() {
_init_virtual_display();
}
int deinit_virtual_display() {
_deinit_virtual_display();
}
int add_virtual_display(int width, int height, void* byte, int* size) {
_add_virtual_display(width,height,byte,size);
}
int remove_virtual_display(int index) {
_remove_virtual_display(index);
}

*/
import "C"
import (
	"fmt"
	"os"
	"os/exec"
	"unsafe"

	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"github.com/winlabs/gowin32"
)




var (
    virtual_displays []*os.Process = []*os.Process{}
)


func execute(dir string,name string, args ...string) {
    cmd := exec.Command(name,args...)
    cmd.Dir = dir
    b,err := cmd.Output()
    if err != nil {
        log.PushLog(dir + " failed : " + err.Error())
    } else {
        log.PushLog(dir + " sucess : " + string(b))
    }
}


var initialized = false
func ActivateVirtualDriver() {
    if !initialized {
        initialized = true
        if C.initlibrary() == 1 {
            panic(fmt.Errorf("failed to load libdisplay.dll"))
        }
    }

    log.PushLog("activating virtual driver")
    execute("./audio",         "./VBCABLE_Setup_x64.exe","-i","-h")
    execute("./display",       "powershell.exe",".\\install.ps1")
    C.init_virtual_display()
}

func DeactivateVirtualDriver() {
    log.PushLog("deactivate virtual driver")
    C.deinit_virtual_display()
    execute("./display",       "powershell.exe",".\\remove.ps1")
}

func StartVirtualDisplay(width,height int) (string,int) {
    buff := make([]byte, 1024)
    var size C.int = 0;
    display := C.add_virtual_display(C.int(width),C.int(height),
                          unsafe.Pointer(&buff[0]),
                          &size)
    log.PushLog("started virtual display %d",display)
    return string(buff[:size]),int(display)
}

func RemoveVirtualDisplay(index int) {
    log.PushLog("remove virtual display %d",index)
    C.remove_virtual_display(C.int(index))
}


func convert(in [32]uint16) string {
    bytes := []byte{}
    for _, v := range in {
        bytes = append(bytes, byte(v))
    }
    return string(bytes)
}

func Displays() []string {
    names := []string{}
    for _, dd := range gowin32.GetAllDisplayDevices() {
        if (dd.StateFlags & gowin32.DisplayDeviceActive) > 0 {
            names = append(names, dd.DeviceName)
        }
    }
    return names
}