package libvirt

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
)

type Volume struct {
	Disposable bool
	Path       string  `yaml:"path"`
	Backing    *Volume `yaml:"backing"`
}

func NewVolume(path ...string) *Volume {
	if len(path) == 0 {
		return nil
	}

	abs, _ := filepath.Abs(path[0])
	child := NewVolume(path[1:]...)
	return &Volume{
		Path:       abs,
		Backing:    child,
		Disposable: true,
	}
}

func (chain *Volume) PushChain(size int) error {
	_, err := os.Stat(chain.Path)
	if err != nil {
		return err
	}

	now := uuid.NewString()
	dir := filepath.Dir(chain.Path)
	path := fmt.Sprintf("%s/child/%s.qcow2", dir, now)
	_, err = exec.Command("qemu-img", "create", "-f", "qcow2", "-F", "qcow2", "-o",
		fmt.Sprintf("backing_file=%s", chain.Path), path,
		fmt.Sprintf("%dG", size)).Output()
	if err != nil {
		return fmt.Errorf("failed to create volume %s",err.Error())
	}

	copy := *chain
	chain.Path = path
	chain.Backing = &copy
	return nil
}

func (volume *Volume) PopChain() error {
	current := volume.Path
	volume.Path = volume.Backing.Path
	volume.Backing = volume.Backing.Backing
	return os.Remove(current)
}

func (volume *Volume) AllFiles() []string {
	if volume == nil {
		return []string{}
	}

	return append(volume.Backing.AllFiles(), volume.Path)
}
