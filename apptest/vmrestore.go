package apptest

import "io"

// StartVmrestore starts an instance of vmrestore with the given flags and waits
// until it exits.
func StartVmrestore(instance, src, storageDataPath string, output io.Writer) error {
	flags := []string{
		"-src=" + src,
		"-storageDataPath=" + storageDataPath,
	}
	_, _, err := startApp(instance, "../../bin/vmrestore", flags, &appOptions{wait: true, output: output})
	return err
}
