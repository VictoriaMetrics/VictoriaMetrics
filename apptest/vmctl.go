package apptest

// Vmctl holds the state of a vmctl app and provides vmctl-specific functions
type Vmctl struct {
	*app
}

// StartVmctl starts an instance of vmctl cli with the given flags
func StartVmctl(instance string, flags []string) (*Vmctl, error) {
	app, _, err := startApp(instance, "../../bin/vmctl", flags, &appOptions{wait: true})
	if err != nil {
		return nil, err
	}

	return &Vmctl{
		app: app,
	}, nil
}
