package apptest

import (
	"io"
	"os"
)

// StartVmctl starts an instance of vmctl cli with the given flags

// StartVmctl starts the latest version of vmctl with the given flags and
// waits until it exits.
//
// The path to the binary can be provided via VMCTL_PATH environment
// variable. If the variable is not set, ../../bin/vmctl-race will be
// used.
func StartVmctl(instance string, flags []string, output io.Writer) error {
	binary := os.Getenv("VMCTL_PATH")
	if binary == "" {
		binary = "../../bin/vmctl-race"
	}
	_, _, err := startApp(instance, binary, flags, &appOptions{wait: true, output: output})
	return err
}
