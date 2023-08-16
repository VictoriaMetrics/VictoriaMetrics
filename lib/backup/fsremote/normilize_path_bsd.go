//go:build freebsd || openbsd || dragonfly || netbsd
// +build freebsd openbsd dragonfly netbsd

package fsremote

func pathToCanonical(path string) string {
	return path
}

func canonicalPathToLocal(path string) string {
	return path
}
