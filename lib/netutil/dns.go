package netutil

import (
	"strings"
)

// ParseGroupAddr parses `groupID/addrX` addr and returns (groupID, addrX).
//
// If addr doesn't contain `groupID/` prefix, then ("", addr) is returned.
func ParseGroupAddr(addr string) (string, string) {
	n := strings.IndexByte(addr, '/')
	if n < 0 {
		return "", addr
	}
	if strings.HasPrefix(addr, "file:") {
		return "", addr
	}
	return addr[:n], addr[n+1:]
}
