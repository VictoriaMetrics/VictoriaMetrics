package atomicutil

import (
	"unsafe"

	"golang.org/x/sys/cpu"
)

// CacheLineSize is the size of a CPU cache line
const CacheLineSize = unsafe.Sizeof(cpu.CacheLinePad{})
