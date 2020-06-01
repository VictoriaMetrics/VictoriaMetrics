package notifier

import (
	"fmt"
	"strings"
)

type errGroup struct {
	errs []string
}

func (eg *errGroup) err() error {
	if eg == nil || len(eg.errs) == 0 {
		return nil
	}
	return eg
}

func (eg *errGroup) Error() string {
	return fmt.Sprintf("errors: %s", strings.Join(eg.errs, "\n"))
}
