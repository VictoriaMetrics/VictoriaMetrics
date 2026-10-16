package fs

import (
	"os"
)

func fadviseSequentialRead(f *os.File, prefetch bool) error {
	// TODO: implement this properly
	return nil
}

func fadviseRandomRead(_ *os.File) error {
	// TODO: implement this properly
	return nil
}
