package fs

import (
	"os"
)

func fadviseSequentialRead(_ *os.File, _ bool) error {
	// TODO: implement this properly
	return nil
}
