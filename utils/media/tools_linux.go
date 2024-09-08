//go:build linux

package media

import (
	"fmt"
)



func ActivateVirtualDriver() {
}

func DeactivateVirtualDriver() {
}

func StartVirtualDisplay(width, height int) (string, int, error) {
	return "", 0, fmt.Errorf("virtual not available for linux")
}

func RemoveVirtualDisplay(index int) {
}

func Displays() []string {
	return []string{}
}
