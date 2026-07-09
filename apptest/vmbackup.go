package apptest

import (
	"io"
	"os"
)

// StartVmbackup starts the latest version of vmbackup with the given flags and
// waits until it exits.
//
// The path to the binary can be provided via VMBACKUP_PATH environment
// variable. If the variable is not set, ../../bin/vmbackup-race will be
// used.
func StartVmbackup(instance, storageDataPath, snapshotCreateURL, dst string, output io.Writer) error {
	binary := os.Getenv("VMBACKUP_PATH")
	if binary == "" {
		binary = "../../bin/vmbackup-race"
	}
	flags := []string{
		"-storageDataPath=" + storageDataPath,
		"-snapshot.createURL=" + snapshotCreateURL,
		"-dst=" + dst,
	}
	_, _, err := startApp(instance, binary, flags, &appOptions{wait: true, output: output})
	return err
}
