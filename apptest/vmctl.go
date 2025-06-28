package apptest

// StartVmctl starts an instance of vmctl cli with the given flags
func StartVmctl(instance string, flags []string) error {
	_, _, err := startApp(instance, "../../bin/vmctl", flags, &appOptions{wait: true})
	return err
}
