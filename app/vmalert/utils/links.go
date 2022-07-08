package utils

import "strings"

const prefix = "/vmalert/"

// Prefix returns "/vmalert/" prefix if it is missing in the path.
func Prefix(path string) string {
	if strings.HasPrefix(path, prefix) {
		return ""
	}
	return prefix
}
