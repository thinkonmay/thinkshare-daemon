package media

/*
#include <Windows.h>
typedef int (*FUNC)	();
typedef int (*FUNC2)	(int width, int height, char* byte, int* size);

static FUNC _init_virtual_display;
static FUNC _deinit_virtual_display;
static FUNC _remove_virtual_display;
static FUNC2 _add_virtual_display;

int
initlibrary() {
	void* hModule 	= LoadLibrary(".\\libdisplay.dll");
	_init_virtual_display 	= (FUNC)	GetProcAddress( hModule,"init_virtual_display");
	_deinit_virtual_display = (FUNC)	GetProcAddress( hModule,"deinit_virtual_display");
	_remove_virtual_display	= (FUNC)	GetProcAddress( hModule,"remove_virtual_display");
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
int remove_virtual_display() {
_remove_virtual_display();
}

*/
import "C"
import (
	"fmt"
	"os"
	"unsafe"
	"os/exec"
)



func init() {
    if C.initlibrary() == 1 {
		panic(fmt.Errorf("failed to load libdisplay.dll"))
	}
}

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
    execute("./display",       "./powershell.exe",".\\instruction.ps1")
    C.init_virtual_display()
}

func DeactivateVirtualDriver() {
    C.deinit_virtual_display()
}

func StartVirtualDisplay(width,height int) string {
    buff := make([]byte, 1024)
    var size C.int = 0;
    C.add_virtual_display(C.int(width),C.int(height),
                          unsafe.Pointer(&buff[0]),
                          &size)
    return string(buff[:size])
}

func RemoveVirtualDisplay() {
    C.remove_virtual_display()
}
