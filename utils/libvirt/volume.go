package libvirt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

type Volume struct {
	Path    string  `yaml:"path"`
	Backing *Volume `yaml:"backing"`
}
func (chain *Volume) PushChain(size int) (error) {
	_, err := os.Stat(chain.Path)
	if err != nil {
		return err
	}

	now := uuid.NewString()
	dir := filepath.Dir(chain.Path)
	path := fmt.Sprintf("%s/%s.qcow2", dir, now)
	_, err = exec.Command("/usr/bin/qemu-img", "create", "-f", "qcow2", "-F", "qcow2", "-o",
		fmt.Sprintf("backing_file=%s", chain.Path), path,
		fmt.Sprintf("%dG", size)).Output()
	if err != nil {
		return err
	}

	copy := *chain
	chain.Path = path
	chain.Backing = &copy
	return nil
}

func (volume *Volume) PopChain() (error) {
	volume.Path = volume.Backing.Path
	volume.Backing = volume.Backing.Backing
	return os.Remove(volume.Path)
}