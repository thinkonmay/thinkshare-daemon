package sharedmemory

/*
#include "smemory.h"
#include <string.h>
*/
import "C"
import (
	"bytes"
	"errors"
	"syscall"
	"unsafe"

	"github.com/thinkonmay/thinkshare-daemon/utils/log"
	"golang.org/x/sys/windows"
)

const (
	Video0 = C.Video0
	Video1 = C.Video1
	Audio  = C.Audio
	Input  = C.Input
)

func memcpy(to,from unsafe.Pointer, size int) {
	C.memcpy(to, from, C.ulonglong(size))
}

type SharedMemory C.SharedMemory

func SetState(mem *SharedMemory, _type int, state bool) {
	if state {
		mem.queues[_type].metadata.active = 1
	} else {
		mem.queues[_type].metadata.active = 0
	}
}
func SetCodec(mem *SharedMemory, _type int, codec int) {
	mem.queues[_type].metadata.codec = C.int(codec)
}
func SetDisplay(mem *SharedMemory, _type int, _display string) {
	display := []byte(_display)
	if len(display) > 0 {
		memcpy(unsafe.Pointer(&mem.queues[_type].metadata.display[0]), unsafe.Pointer(&display[0]), len(display))
	}
}

func byteSliceToString(s []byte) string {
	n := bytes.IndexByte(s, 0)
	if n >= 0 {
		s = s[:n]
	}
	return string(s)
}

func AllocateSharedMemory() (*SharedMemory, string, func(), error) {
	mod, err := syscall.LoadDLL("libparent.dll")
	if err != nil {
		return nil, "", func() {}, err
	}
	deinit, err := mod.FindProc("deinit_shared_memory")
	if err != nil {
		return nil, "", func() {}, err
	}
	allocate, err := mod.FindProc("allocate_shared_memory")
	if err != nil {
		deinit.Call()
		return nil, "", func() {}, err
	}

	buffer := make([]byte, 128)
	_, _, err = allocate.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
	)
	if !errors.Is(err, windows.ERROR_SUCCESS) {
		deinit.Call()
		return nil, "", func() {}, err
	}

	obtain, err := mod.FindProc("obtain_shared_memory")
	if err != nil {
		deinit.Call()
		return nil, "", func() {}, err
	}

	pointer, _, err := obtain.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
	)
	if !errors.Is(err, windows.ERROR_SUCCESS) {
		deinit.Call()
		return nil, "", func() {}, err
	}

	return (*SharedMemory)(unsafe.Pointer(pointer)),
		byteSliceToString(buffer),
		func() {
			log.PushLog("deallocated shared memory")
			deinit.Call()
		},
		nil
}
