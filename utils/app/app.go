package apps

import "os/exec"

func StartApp(path string, args ...string) {
	exec.Command(path,args...).Start()
}
