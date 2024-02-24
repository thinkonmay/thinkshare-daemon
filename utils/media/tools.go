package media

import (
	"fmt"
	"os"
	"os/exec"
	"unsafe"

)

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
