package fs

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/sys/windows"
)

var (
	kernelDLL            = windows.MustLoadDLL("kernel32.dll")
	procLock             = kernelDLL.MustFindProc("LockFileEx")
	procEvent            = kernelDLL.MustFindProc("CreateEventW")
	procDisk             = kernelDLL.MustFindProc("GetDiskFreeSpaceExW")
	ntDLL                = windows.MustLoadDLL("ntdll.dll")
	ntSetInformationProc = ntDLL.MustFindProc("NtSetInformationFile")
)

// panic at windows.
// https://github.com/dgraph-io/badger/issues/699
// https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers
func MustSyncPath(string) {
}

const (
	lockfileExclusiveLock = 2
	fileFlagNormal        = 0x00000080
	// https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/ntddk/ns-ntddk-_file_disposition_information_ex
	FILE_DISPOSITION_DELETE                    = 0x00000001
	FILE_DISPOSITION_POSIX_SEMANTICS           = 0x00000002
	FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE = 0x00000010
)

// CreateFlockFile creates flock.lock file in the directory dir
// and returns the handler to the file.
// https://github.com/juju/fslock/blob/master/fslock_windows.go
func CreateFlockFile(dir string) (*os.File, error) {
	flockFile := dir + "/flock.lock"
	name, err := windows.UTF16PtrFromString(flockFile)
	if err != nil {
		return nil, err
	}
	// TODO share_delete is hack. need to fix posix  set
	// or test it properly.
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
	// overlapped is dropped?
	r1, _, err := procLock.Call(uintptr(handle), uintptr(lockfileExclusiveLock), uintptr(0), uintptr(1), uintptr(0), uintptr(unsafe.Pointer(ol)))
	if r1 == 0 {
		return nil, err
	}
	if err := setPosixDelete(handle); err != nil {
		return nil, err
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

// FILE_DISPOSITION_INFORMATION_EX windows information scheme
// https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/ntddk/ns-ntddk-_file_disposition_information_ex
type FILE_DISPOSITION_INFORMATION_EX struct {
	Flags uint32
}
type ioStatusBlock struct {
	Status, Information uintptr
}

// supported starting with Windows 10, version 1709.
func setPosixDelete(handle windows.Handle) error {
	var iosb ioStatusBlock
	// class FileDispositionInformationEx,                   // 64
	// https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/wdm/ne-wdm-_file_information_class
	flags := FILE_DISPOSITION_INFORMATION_EX{
		Flags: FILE_DISPOSITION_DELETE | FILE_DISPOSITION_POSIX_SEMANTICS | FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE,
	}
	r0, _, err := ntSetInformationProc.Call(uintptr(handle), uintptr(unsafe.Pointer(&iosb)), uintptr(unsafe.Pointer(&flags)), unsafe.Sizeof(flags), uintptr(64))
	if r0 == 0 {
		return fmt.Errorf("cannot set file disposition information: %w", err)
	}
	if r0 == 0xC000000D {
		logger.Infof("invalid parametr response from windows: %X, %v", r0, err)
	}
	return nil
}

// UpdateFileHandle - changes file deletion semantic at windows to posix-like.
func UpdateFileHandle(path string) error {
	handle, err := windows.Open(path, windows.GENERIC_READ, windows.FILE_SHARE_READ)
	if err != nil {
		return err
	}
	return setPosixDelete(handle)
}
