package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/update"
)

var (
	proj 	 = "avmvymkexjarplbxwlnj"
	anon_key = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6ImF2bXZ5bWtleGphcnBsYnh3bG5qIiwicm9sZSI6ImFub24iLCJpYXQiOjE2ODAzMjM0NjgsImV4cCI6MTk5NTg5OTQ2OH0.y2W9svI_4O4_xd5AQk4S4MLJAvQJIp0QrO4cljLB9Ik"
)
func init() {
	project := os.Getenv("TM_PROJECT")
	key     := os.Getenv("TM_ANONKEY")
	if project != "" {
		proj = project
	}
	if key != "" {
		anon_key = key
	}
}

func main() {
	credential.SetupEnv(proj, anon_key)
	update.Update()
	cmd := exec.Command("./daemon.exe")
	cmd.Dir = "./package"
	cmd.Stdout 	= os.Stdout
    cmd.Stderr 	= os.Stderr
	cmd.Stdin 	= os.Stdin
	if os.Getenv("VIRTUAL_DISPLAY") == "TRUE" {
		go func() {
			out,err := exec.Command("./display/IndirectDisplay/x64/Release/IddSampleApp.exe").Output()
			if err != nil {
				fmt.Printf("failed to start virtual display %s %s\n", err.Error(),out)
			}
		}()
	}
	cmd.Run()
}