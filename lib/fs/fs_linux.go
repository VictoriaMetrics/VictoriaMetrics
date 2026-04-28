//go:build linux

package fs

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
	0xadf5:     "adfs",
	0xadff:     "affs",
	0x5346414F: "afs",
	0x0187:     "autofs",
	0xf15f:     "ecryptfs",
	0x414A53:   "efs",
	0xE0F5E1E2: "erofs",
	0x6969:     "nfs",
	0xFF534D42: "cifs",
	0x6c6f6f70: "binderfs",
	0xBAD1DEA:  "futexfs",
}

func getFsType(path string) string {
	var stat statfs_t
	fsName := "unknown"
	err := statfs(path, &stat)
	if err != nil {
		return fsName
	}
	if fsn, ok := fsMagicNumberToName[int64(stat.Type)]; ok {
		fsName = fsn
	}
	return fsName
}
