package apptest

import "fmt"

const vmctlAppName = "vmctl"

// Vmctl holds the state of a vmctl app and provides vmctl-specific functions
type Vmctl struct {
	*app
}

// StartVmctl starts an instance of vmctl cli with the given flags. It also
// populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVmctl(instance string, flags []string) (*Vmctl, error) {

	app, _, err := startApp(instance, "../../bin/vmctl", flags, &appOptions{})
	if err != nil {
		return nil, err
	}

	return &Vmctl{
		app: app,
	}, nil
}

// Wait waits for the vmctl process to finish and returns an error if it
// exited with a non-zero exit code or if it failed to start.
func (vmctl *Vmctl) Wait() error {
	if vmctl.process == nil {
		return nil
	}
	for {
		state, err := vmctl.process.Wait()
		if err != nil {
			return fmt.Errorf("vmctl process %s failed: %w", vmctl.instance, err)
		}
		if state.Success() {
			return nil
		}
		if state.Exited() {
			return fmt.Errorf("vmctl process %s exited with code %d", vmctl.instance, state.ExitCode())
		}
		continue
	}
}
