//go:build windows

package childprocess

import "syscall"

var (
	attr = &syscall.SysProcAttr{HideWindow: true}
)
