package formatter

import (
	"bytes"
	"fmt"

	"github.com/mgechev/revive/lint"
)

// Default is an implementation of the Formatter interface
// which formats the errors to text.
type Default struct {
	Metadata lint.FormatterMetadata
}

// Name returns the name of the formatter
func (*Default) Name() string {
	return "default"
}

// Format formats the failures gotten from the lint.
func (*Default) Format(failures <-chan lint.Failure, _ lint.Config) (string, error) {
	var buf bytes.Buffer
	for failure := range failures {
		fmt.Fprintf(&buf, "%v: %s\n", failure.Position.Start, failure.Failure)
	}
	return buf.String(), nil
}

func ruleDescriptionURL(ruleName string) string {
	return "https://github.com/mgechev/revive/blob/master/RULES_DESCRIPTIONS.md#" + ruleName
}
