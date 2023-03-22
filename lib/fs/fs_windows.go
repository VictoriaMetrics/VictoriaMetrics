package fs

import (
	"fmt"
	"os"
	"reflect"
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/windows"
)

var (
	kernelDLL = windows.MustLoadDLL("kernel32.dll")
	procLock  = kernelDLL.MustFindProc("LockFileEx")
	procEvent = kernelDLL.MustFindProc("CreateEventW")
	procDisk  = kernelDLL.MustFindProc("GetDiskFreeSpaceExW")
)

func mustSyncPath(path string) {
}

const (
	lockfileExclusiveLock = 2
	fileFlagNormal        = 0x00000080
)

// https://github.com/juju/fslock/blob/master/fslock_windows.go
func createFlockFile(flockFile string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(flockFile)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ|windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_ALWAYS,
		windows.FILE_FLAG_OVERLAPPED|fileFlagNormal,
		0)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file %q: %w", flockFile, err)
	}
	ol, err := newOverlapped()
	if err != nil {
		return nil, fmt.Errorf("cannot create Overlapped handler: %w", err)
	}
	// https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex
	r1, _, err := procLock.Call(uintptr(handle), uintptr(lockfileExclusiveLock), uintptr(0), uintptr(1), uintptr(0), uintptr(unsafe.Pointer(ol)))
	if r1 == 0 {
		return nil, err
	}
	return os.NewFile(uintptr(handle), flockFile), nil
}

var (
	fileMappingMU     sync.Mutex
	fileMappingByAddr = map[uintptr]windows.Handle{}
)

func mmap(fd int, length int) ([]byte, error) {
	flProtect := uint32(windows.PAGE_READONLY)
	dwDesiredAccess := uint32(windows.FILE_MAP_READ)
	// https://learn.microsoft.com/en-us/windows/win32/memory/creating-a-file-mapping-object#file-mapping-size
	// do not specify any length params, windows will set it according to the file size.
	// If length > file size, truncate is required according to api definition, we don't want it.
	h, errno := windows.CreateFileMapping(windows.Handle(fd), nil, flProtect, 0, 0, nil)
	if h == 0 {
		return nil, os.NewSyscallError("CreateFileMapping", errno)
	}
	addr, errno := windows.MapViewOfFile(h, dwDesiredAccess, 0, 0, 0)
	if addr == 0 {
		windows.CloseHandle(h)
		return nil, os.NewSyscallError("MapViewOfFile", errno)
	}
	fileMappingMU.Lock()
	fileMappingByAddr[addr] = h
	fileMappingMU.Unlock()
	data := make([]byte, 0)
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	hdr.Data = addr
	hdr.Len = length
	hdr.Cap = hdr.Len

	return data, nil
}

func mUnmap(data []byte) error {
	// flush is not needed, since we perform only reading operation.
	// In case of write, additional call FlushViewOfFile must be performed.
	header := (*reflect.SliceHeader)(unsafe.Pointer(&data))
	addr := header.Data
	fileMappingMU.Lock()
	defer fileMappingMU.Unlock()
	if err := windows.UnmapViewOfFile(addr); err != nil {
		return err
	}

	handle, ok := fileMappingByAddr[addr]
	if !ok {
		logger.Fatalf("BUG: unmapping for non exist addr: %d", addr)
	}
	delete(fileMappingByAddr, addr)

	e := windows.CloseHandle(handle)
	return os.NewSyscallError("CloseHandle", e)
}

func mustGetFreeSpace(path string) uint64 {
	var freeBytes int64
	r, _, err := procDisk.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(path))),
		uintptr(unsafe.Pointer(&freeBytes)))
	if r == 0 {
		logger.Errorf("cannot get free space for path: %q : %s", path, err)
		return 0
	}
	return uint64(freeBytes)
}

// stub
func fadviseSequentialRead(f *os.File, prefetch bool) error {
	return nil
}

// copied from https://github.com/juju/fslock/blob/master/fslock_windows.go
// https://docs.microsoft.com/en-us/windows/win32/api/minwinbase/ns-minwinbase-overlapped
func newOverlapped() (*windows.Overlapped, error) {
	event, err := createEvent(nil, nil)
	if err != nil {
		return nil, err
	}
	return &windows.Overlapped{HEvent: event}, nil
}

// copied from https://github.com/juju/fslock/blob/master/fslock_windows.go
// https://docs.microsoft.com/en-us/windows/win32/api/synchapi/nf-synchapi-createeventa
func createEvent(sa *windows.SecurityAttributes, name *uint16) (windows.Handle, error) {
	r0, _, err := procEvent.Call(uintptr(unsafe.Pointer(sa)), uintptr(1), uintptr(1), uintptr(unsafe.Pointer(name)))
	handle := windows.Handle(r0)
	if handle == windows.InvalidHandle {
		return 0, err
	}
	return handle, nil
}
