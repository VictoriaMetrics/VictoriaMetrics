package fs

import (
	"fmt"
	"os"
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/windows"
)

// at windows only files could be synced
// Sync for directories is not supported.
func mustSyncPath(_ string) {
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
	err = windows.LockFileEx(handle, lockfileExclusiveLock, 0, 0, 0, ol)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), flockFile), nil
}

var (
	mmapByAddrLock sync.Mutex
	mmapByAddr     = map[uintptr]windows.Handle{}
)

func mmap(fd, length int) ([]byte, error) {
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

	// mitigate go vet false positive
	// https://github.com/golang/go/issues/58625
	addrPtr := *(*unsafe.Pointer)(unsafe.Pointer(&addr))
	data := unsafe.Slice((*byte)(addrPtr), length)

	mmapByAddrLock.Lock()
	mmapByAddr[addr] = h
	mmapByAddrLock.Unlock()

	return data, nil
}

func mUnmap(data []byte) error {
	// flush is not needed, since we perform only reading operation.
	// In case of write, additional call FlushViewOfFile must be performed.
	addr := uintptr(unsafe.Pointer(unsafe.SliceData(data)))

	mmapByAddrLock.Lock()
	h, ok := mmapByAddr[addr]
	if !ok {
		logger.Panicf("BUG: unmapping for non exist addr: %d", addr)
	}
	delete(mmapByAddr, addr)
	mmapByAddrLock.Unlock()

	if err := windows.UnmapViewOfFile(addr); err != nil {
		return fmt.Errorf("cannot unmap memory mapped file: %w", err)
	}
	errno := windows.CloseHandle(h)
	return os.NewSyscallError("CloseHandle", errno)
}

func mustGetFreeSpace(path string) uint64 {
	var freeBytes uint64
	// https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getdiskfreespaceexw
	err := windows.GetDiskFreeSpaceEx(windows.StringToUTF16Ptr(path), &freeBytes, nil, nil)
	if err != nil {
		logger.Panicf("FATAL: cannot get free space for %q : %s", path, err)
	}
	return freeBytes
}

// stub
func fadviseSequentialRead(_ *os.File, _ bool) error {
	return nil
}

// https://docs.microsoft.com/en-us/windows/win32/api/minwinbase/ns-minwinbase-overlapped
func newOverlapped() (*windows.Overlapped, error) {
	event, err := windows.CreateEvent(nil, 1, 1, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create event: %w", err)
	}
	return &windows.Overlapped{HEvent: event}, nil
}
