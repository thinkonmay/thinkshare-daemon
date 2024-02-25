package system

import (
	"fmt"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSysinf(t *testing.T) {
	inf,err := GetInfor()
	if err != nil {
		t.Error(err)
		return 
	}
	out,_ := yaml.Marshal(inf)
	fmt.Printf("%s",string(out))
}