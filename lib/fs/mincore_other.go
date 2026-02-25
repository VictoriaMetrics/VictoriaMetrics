//go:build !linux

package fs

import (
	"fmt"
)

func supportsMincore() bool {
	return false
}

func mincore(ptr *byte) bool {
	panic(fmt.Errorf("BUG: unexpected call"))
}
