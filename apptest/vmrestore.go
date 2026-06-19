package apptest

import (
	"io"
	"os"
)

// StartVmrestore starts the latest version of vmrestore with the given flags
// and waits until it exits.
//
// The path to the binary can be provided via VMRESTORE_PATH environment
// variable. If the variable is not set, ../../bin/vmrestore-race will be
// used.
func StartVmrestore(instance, src, storageDataPath string, output io.Writer) error {
	binary := os.Getenv("VMRESTORE_PATH")
	if binary == "" {
		binary = "../../bin/vmrestore-race"
	}
	flags := []string{
		"-src=" + src,
		"-storageDataPath=" + storageDataPath,
	}
	_, _, err := startApp(instance, binary, flags, &appOptions{wait: true, output: output})
	return err
}
