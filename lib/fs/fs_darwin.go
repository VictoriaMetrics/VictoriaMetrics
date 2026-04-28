//go:build darwin

package fs

func getFsType(_ string) string {
	return "unknown"
}
