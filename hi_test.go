package daemon

import (
	"fmt"
	"testing"
)

func TestDaemon(t *testing.T) {
	fmt.Println(base_dir)
	fmt.Println(child_dir)
	outs, err := listVolumesInDir("./binary")
	if err != nil {
		panic(err)
	}

	for _, out := range outs {
		path, err := findVolumesInDir("./binary", out)
		if err != nil {
			panic(err)
		}

		fmt.Printf("%v\n", path)
	}
}
