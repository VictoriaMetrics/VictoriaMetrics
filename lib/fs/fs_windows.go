package fs

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/windows"
)

var (
	kernelDLL = windows.MustLoadDLL("kernel32.dll")
	procLock  = kernelDLL.MustFindProc("LockFileEx")
	prcEvent  = kernelDLL.MustFindProc("CreateEventW")
	procDisk  = kernelDLL.MustFindProc("GetDiskFreeSpaceExW")
)

// panic at windows.
// https://github.com/dgraph-io/badger/issues/699
// https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers
func MustSyncPath(string) {
}

// CreateFlockFile creates flock.lock file in the directory dir
// and returns the handler to the file.
// https://github.com/juju/fslock/blob/master/fslock_windows.go
func CreateFlockFile(dir string) (*os.File, error) {
	flockFile := dir + "/flock.lock"
	name, err := syscall.UTF16PtrFromString(flockFile)
	if err != nil {
		return nil, err
	}
	handle, err := syscall.CreateFile(
		name,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ,
		nil,
		syscall.OPEN_ALWAYS,
		syscall.FILE_FLAG_OVERLAPPED|0x00000080,
		0)
	if err != nil {
		return nil, fmt.Errorf("cannot create lock file %q: %w", flockFile, err)
	}
	ol, err := newOverlapped()
	if err != nil {
		return nil, fmt.Errorf("cannot create Overlapped handler: %w", err)
	}
	// first argument is result.
	r1, _, e1 := syscall.Syscall6(procLock.Addr(), 6, uintptr(handle), uintptr(2), uintptr(0), uintptr(1), uintptr(0), uintptr(unsafe.Pointer(ol)))
	if r1 == 0 {
		if e1 != 0 {
			return nil, error(e1)
		} else {
			return nil, syscall.EINVAL
		}
	}
	return os.NewFile(uintptr(handle), flockFile), nil
}

// stub
func mmap(fd int, offset int64, length int) ([]byte, error) {
	return nil, nil
}

// stub
func munMap([]byte) error {
	return nil
}

func mustGetFreeSpace(path string) uint64 {
	var freeBytes int64
	r, _, err := procDisk.Call(uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(path))),
		uintptr(unsafe.Pointer(&freeBytes)))
	if r == 0 {
		logger.Errorf("cannot get free spacE: %v", err)
		return 0
	}
	return uint64(freeBytes)
}

func fadviseSequentialRead(f *os.File, prefetch bool) error {
	return nil
}

// copied from https://github.com/juju/fslock/blob/master/fslock_windows.go
func newOverlapped() (*syscall.Overlapped, error) {
	event, err := createEvent(nil, true, false, nil)
	if err != nil {
		return nil, err
	}
	return &syscall.Overlapped{HEvent: event}, nil
}

// damn magic
// copied from https://github.com/juju/fslock/blob/master/fslock_windows.go
func createEvent(sa *syscall.SecurityAttributes, manualReset bool, initialState bool, name *uint16) (handle syscall.Handle, err error) {
	var _p0 uint32
	if manualReset {
		_p0 = 1
	}
	var _p1 uint32
	if initialState {
		_p1 = 1
	}
	r0, _, e1 := syscall.Syscall6(prcEvent.Addr(), 4, uintptr(unsafe.Pointer(sa)), uintptr(_p0), uintptr(_p1), uintptr(unsafe.Pointer(name)), 0, 0)
	handle = syscall.Handle(r0)
	if handle == syscall.InvalidHandle {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}
