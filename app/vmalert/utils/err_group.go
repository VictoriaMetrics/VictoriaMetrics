package utils

import (
	"fmt"
	"strings"
	"sync"
)

// ErrGroup accumulates multiple errors
// and produces single error message.
type ErrGroup struct {
	mu   sync.Mutex
	errs []error
}

// Add adds a new error to group.
// Is thread-safe.
func (eg *ErrGroup) Add(err error) {
	eg.mu.Lock()
	eg.errs = append(eg.errs, err)
	eg.mu.Unlock()
}

// Err checks if group contains at least
// one error.
func (eg *ErrGroup) Err() error {
	if eg == nil {
		return nil
	}

	eg.mu.Lock()
	defer eg.mu.Unlock()
	if len(eg.errs) == 0 {
		return nil
	}
	return eg
}

// Error satisfies Error interface
func (eg *ErrGroup) Error() string {
	eg.mu.Lock()
	defer eg.mu.Unlock()

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
