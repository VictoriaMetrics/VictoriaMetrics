package fsremote

import "strings"

func pathToCanonical(path string) string {
	return strings.ReplaceAll(path, "\\", "/")
}

func canonicalPathToLocal(path string) string {
	return strings.ReplaceAll(path, "/", "\\")
}
