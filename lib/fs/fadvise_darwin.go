package fs

import (
	"os"
)

func fadviseSequentialRead(f *os.File, prefetch bool) error { // nolint
	// TODO: implement this properly
	return nil
}
