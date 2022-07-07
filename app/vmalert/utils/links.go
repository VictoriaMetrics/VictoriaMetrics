package utils

import "strings"

const prefix = "/vmalert/"

func Prefix(path string) string {
	if strings.HasPrefix(path, prefix) {
		return ""
	}
	return prefix
}
