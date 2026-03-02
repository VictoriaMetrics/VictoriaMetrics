//go:build linux || darwin || freebsd

package fs

import (
	"sync"

	"golang.org/x/sys/unix"
)

// https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/tree/include/uapi/linux/magic.h
var fsMagicNumberToName = map[int64]string{
	0xEF53:     "ext2/ext3/ext4",
	0xabba1974: "xenfs",
	0x9123683E: "btrfs",
	0x3434:     "nilfs",
	0xF2F52010: "f2fs",
	0xf995e849: "hpfs",
	0x9660:     "isofs",
	0x72b6:     "jffs2",
	0x58465342: "xfs",
	0x6165676C: "pstorefs",
	0xde5e81e4: "efivarfs",
	0x00c0ffee: "hostfs",
	0x794c7630: "overlayfs",
	0x65735546: "fuse",
	0xca451a4e: "bcachefs",
}

// Path -> Fs Type
var lock sync.Mutex
var fsNameCache = map[string]string{}

type statfs_t = unix.Statfs_t

func freeSpace(stat statfs_t) uint64 {
	return uint64(stat.Bavail) * uint64(stat.Bsize)
}

// totalSpace returns the total capacity of the filesystem described by stat in bytes.
func totalSpace(stat statfs_t) uint64 {
	return uint64(stat.Blocks) * uint64(stat.Bsize)
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
	if fsn, ok := fsMagicNumberToName[int64(stat.Type)]; ok {
		fsName = fsn
	}

	lock.Lock()
	fsNameCache[path] = fsName
	lock.Unlock()

	return fsName
}
