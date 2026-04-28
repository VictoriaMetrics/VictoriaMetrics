//go:build freebsd

package fs

func getFsType(_ string) string {
	return "unknown"
}
