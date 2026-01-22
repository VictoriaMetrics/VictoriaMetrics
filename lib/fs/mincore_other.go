//go:build !linux

package fs

import (
	"fmt"
)

var mincoreSupported = false

func mincore(ptr *byte) bool {
	panic(fmt.Errorf("BUG: unexpected call"))
}
