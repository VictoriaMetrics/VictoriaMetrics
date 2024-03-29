package gomonkey

import (
	"fmt"
	"reflect"
	"syscall"
	"unsafe"
)

func PtrOf(val []byte) uintptr {
	return (*reflect.SliceHeader)(unsafe.Pointer(&val)).Data
}

func modifyBinary(target uintptr, bytes []byte) {
	targetPage := pageStart(target)
	res := write(target, PtrOf(bytes), len(bytes), targetPage, syscall.Getpagesize(), syscall.PROT_READ|syscall.PROT_EXEC)
	if res != 0 {
		panic(fmt.Errorf("failed to write memory, code %v", res))
	}
}

//go:cgo_import_dynamic mach_task_self mach_task_self "/usr/lib/libSystem.B.dylib"
//go:cgo_import_dynamic mach_vm_protect mach_vm_protect "/usr/lib/libSystem.B.dylib"
func write(target, data uintptr, len int, page uintptr, pageSize, oriProt int) int
