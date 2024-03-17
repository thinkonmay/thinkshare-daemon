//go:build linux

package childprocess

import "syscall"


var (
	attr = &syscall.SysProcAttr{}
)