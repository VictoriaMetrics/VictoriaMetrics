package formatter

import (
	"fmt"
	"strings"

	"github.com/mgechev/revive/lint"
)

// Unix is an implementation of the Formatter interface
// which formats the errors to a simple line based error format
//
//	main.go:24:9: [errorf] should replace errors.New(fmt.Sprintf(...)) with fmt.Errorf(...)
type Unix struct {
	Metadata lint.FormatterMetadata
}

// Name returns the name of the formatter
func (*Unix) Name() string {
	return "unix"
}

// Format formats the failures gotten from the lint.
func (*Unix) Format(failures <-chan lint.Failure, _ lint.Config) (string, error) {
	var sb strings.Builder
	for failure := range failures {
		sb.WriteString(fmt.Sprintf("%v: [%s] %s\n", failure.Position.Start, failure.RuleName, failure.Failure))
	}
	return sb.String(), nil
}
