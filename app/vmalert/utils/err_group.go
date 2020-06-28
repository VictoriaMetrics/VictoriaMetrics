package utils

import (
	"fmt"
	"strings"
)

// ErrGroup accumulates multiple errors
// and produces single error message.
type ErrGroup struct {
	errs []error
}

// Add adds a new error to group.
// Isn't thread-safe.
func (eg *ErrGroup) Add(err error) {
	eg.errs = append(eg.errs, err)
}

// Err checks if group contains at least
// one error.
func (eg *ErrGroup) Err() error {
	if eg == nil || len(eg.errs) == 0 {
		return nil
	}
	return eg
}

// Error satisfies Error interface
func (eg *ErrGroup) Error() string {
	if len(eg.errs) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "errors(%d): ", len(eg.errs))
	for i, err := range eg.errs {
		b.WriteString(err.Error())
		if i != len(eg.errs)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}
