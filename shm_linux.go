package daemon

/*
#include "smemory.h"
#include <string.h>
*/
import "C"
import (
	"bytes"
	"unsafe"
	"github.com/ebitengine/purego"

	"github.com/thinkonmay/thinkshare-daemon/utils/log"

)

const (
	Video0 = C.Video0
	Video1 = C.Video1
	Audio  = C.Audio
	Input  = C.Input
)

type SharedMemory C.SharedMemory

func byteSliceToString(s []byte) string {
	n := bytes.IndexByte(s, 0)
	if n >= 0 {
		s = s[:n]
	}
	return string(s)
}

func AllocateSharedMemory() (*SharedMemory, string, func(), error) {
	mod, err := purego.Dlopen("./libparent.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, "", func() {}, err
	}
	var deinit func()
	purego.RegisterLibFunc(&deinit, mod, "deinit_shared_memory")

	var allocate func(unsafe.Pointer) unsafe.Pointer
	buffer := make([]byte, 128)
	purego.RegisterLibFunc(&allocate, mod, "allocate_shared_memory")
	pointer := allocate(unsafe.Pointer(&buffer[0]))

	return (*SharedMemory)(unsafe.Pointer(pointer)),
		byteSliceToString(buffer),
		func() {
			log.PushLog("deallocated shared memory")
			deinit()		
		},
		nil
}
