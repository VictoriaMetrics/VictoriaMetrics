package apptest

// StartVmbackup starts an instance of vmbackup with the given flags and waits
// until it exits.
func StartVmbackup(instance, storageDataPath, snapshotCreateURL, dst string) error {
	flags := []string{
		"-storageDataPath=" + storageDataPath,
		"-snapshot.createURL=" + snapshotCreateURL,
		"-dst=" + dst,
	}
	_, _, err := startApp(instance, "../../bin/vmbackup", flags, &appOptions{wait: true})
	return err
}
