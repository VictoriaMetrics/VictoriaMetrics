//go:build darwin || freebsd || netbsd || openbsd

package fs

func getFsType(_ string) string {
	return "unknown"
}
