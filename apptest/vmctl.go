package apptest

import "io"

// StartVmctl starts an instance of vmctl cli with the given flags
func StartVmctl(instance string, flags []string, output io.Writer) error {
	_, _, err := startApp(instance, "../../bin/vmctl-race", flags, &appOptions{wait: true, output: output})
	return err
}
