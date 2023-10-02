package main

import (
	"os"
	"os/exec"

	"github.com/thinkonmay/thinkshare-daemon/credential"
	"github.com/thinkonmay/thinkshare-daemon/update"
)

var (
	proj 	 = "https://supabase.thinkmay.net"
	anon_key = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.ewogICJyb2xlIjogImFub24iLAogICJpc3MiOiAic3VwYWJhc2UiLAogICJpYXQiOiAxNjk0MDE5NjAwLAogICJleHAiOiAxODUxODcyNDAwCn0.EpUhNso-BMFvAJLjYbomIddyFfN--u-zCf0Swj9Ac6E"
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
	cmd.Run()
}