package fs

import (
	"golang.org/x/sys/unix"
	"sync"
)

// Path -> Fs Type
var lock sync.Mutex
var fsNameCache = map[string]string{}

type statfs_t = unix.Statfs_t

func freeSpace(stat statfs_t) uint64 {
	return uint64(stat.F_bavail) * uint64(stat.F_bsize)
}

// totalSpace returns the total capacity of the filesystem in bytes.
func totalSpace(stat statfs_t) uint64 {
	return uint64(stat.F_blocks) * uint64(stat.F_bsize)
}

func statfs(path string, stat *statfs_t) (err error) {
	return unix.Statfs(path, stat)
}

func getFsName(path string) string {
	// fast path: get fs name from cache
	lock.Lock()
	if fsName, ok := fsNameCache[path]; ok {
		lock.Unlock()
		return fsName
	}
	lock.Unlock()

	// slow path: get fs name by statfs syscall
	var stat statfs_t
	fsName := "unknown"
	err := statfs(path, &stat)
	if err != nil {
		return fsName
	}
	fsNameBytes := make([]byte, 0, len(stat.F_fstypename))
	for _, v := range stat.F_fstypename {
		if v == 0 {
			break
		}
		fsNameBytes = append(fsNameBytes, byte(v))
	}
	fsName = string(fsNameBytes)
	lock.Lock()
	fsNameCache[path] = fsName
	lock.Unlock()

	return fsName
}
