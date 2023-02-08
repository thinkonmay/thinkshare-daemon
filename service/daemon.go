package service

import (
	"fmt"
	"os/exec"
	"strings"

	childprocess "github.com/thinkonmay/thinkshare-daemon/child-process"
)

type Daemon struct {
	LogURL                 string
	SessionSettingURL      string
	SessionRegistrationURL string

	ServerToken  string
	SessionToken string

	HIDport int

	Childprocess *childprocess.ChildProcesses
	Shutdown     chan bool
}

func NewDaemon(domain string) *Daemon {
	return &Daemon{
		Shutdown:               make(chan bool),
		ServerToken:            "none",
		SessionToken:           "none",
		SessionRegistrationURL: fmt.Sprintf("https://%s/api/worker", domain),
		SessionSettingURL:      fmt.Sprintf("https://%s/api/session/setting", domain),
		LogURL:                 fmt.Sprintf("https://%s/api/log/worker", domain),
		Childprocess:           childprocess.NewChildProcessSystem(),
	}
}


func FindProcessPath(dir *string, process string) (string, error) {
	cmd := exec.Command("where.exe", process)

	if dir != nil {
		cmd.Dir = *dir
	}

	bytes, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	paths := strings.Split(string(bytes), "\n")
	pathss := strings.Split(paths[0], "\r")
	return pathss[0], nil
}

