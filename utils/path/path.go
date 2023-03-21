package path

import (
	"os/exec"
	"strings"
)

func FindProcessPath(dir string, process string) (string, error) {
	cmd := exec.Command("where.exe", process)

	if dir != "" {
		cmd.Dir = dir
	}

	bytes, err := cmd.Output()
	if err != nil {
		return "", nil
	}
	paths := strings.Split(string(bytes), "\n")
	pathss := strings.Split(paths[0], "\r")
	return pathss[0], nil
}