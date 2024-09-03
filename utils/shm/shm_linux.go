package sharedmemory

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
	Video0 int = C.Video0
	Video1 int = C.Video1
	Audio  int = C.Audio
	Input  int = C.Input
)

type SharedMemory C.SharedMemory
type Queue C.Queue

func GetState(mem *SharedMemory, _type int) int {
	return int(mem.queues[_type].metadata.active)
}
func SetState(mem *SharedMemory, _type int, state C.int) {
	mem.queues[_type].metadata.active = state
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

func memcpy(to, from unsafe.Pointer, size int) {
	C.memcpy(to, from, C.ulong(size))
}

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
