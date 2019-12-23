package fs

import (
	"os"
)

// MustFadviseSequentialRead hints the OS that f is read mostly sequentially.
//
// if prefetch is set, then the OS is hinted to prefetch f data.
func MustFadviseSequentialRead(f *os.File, prefetch bool) {
	// TODO: implement this properly
}
