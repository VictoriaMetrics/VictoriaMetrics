package apptest

import (
	"io"
	"os"
)

// StartVmrestore starts the latest version of vmrestore with the given flags
// and waits until it exits.
//
// Additional command-line flags may be passed via extraFlags, e.g. -restorePartitions.
//
// The path to the binary can be provided via VMRESTORE_PATH environment
// variable. If the variable is not set, ../../bin/vmrestore-race will be
// used.
func StartVmrestore(instance, src, storageDataPath string, output io.Writer, extraFlags ...string) error {
	binary := os.Getenv("VMRESTORE_PATH")
	if binary == "" {
		binary = "../../bin/vmrestore-race"
	}
	flags := []string{
		"-src=" + src,
		"-storageDataPath=" + storageDataPath,
	}
	flags = append(flags, extraFlags...)
	_, _, err := startApp(instance, binary, flags, &appOptions{wait: true, output: output})
	return err
}
