package path

import (
	"fmt"
	"os"
	"path/filepath"
)

var (
	dir string
)

func init() {
	exe, _ := os.Executable()
	dir, _ = filepath.Abs(filepath.Dir(exe))
}

func FindProcessPath(process string) (string, error) {
	return filepath.Abs(fmt.Sprintf("%s/%s", dir, process))
}
