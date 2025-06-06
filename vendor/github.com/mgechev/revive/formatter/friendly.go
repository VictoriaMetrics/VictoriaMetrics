package formatter

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/fatih/color"
	"github.com/mgechev/revive/lint"
	"github.com/olekukonko/tablewriter"
)

func getErrorEmoji() string {
	return color.RedString("✘")
}

func getWarningEmoji() string {
	return color.YellowString("⚠")
}

// Friendly is an implementation of the Formatter interface
// which formats the errors to JSON.
type Friendly struct {
	Metadata lint.FormatterMetadata
}

// Name returns the name of the formatter
func (*Friendly) Name() string {
	return "friendly"
}

// Format formats the failures gotten from the lint.
func (f *Friendly) Format(failures <-chan lint.Failure, config lint.Config) (string, error) {
	var buf strings.Builder
	errorMap := map[string]int{}
	warningMap := map[string]int{}
	totalErrors := 0
	totalWarnings := 0
	for failure := range failures {
		sev := severity(config, failure)
		f.printFriendlyFailure(&buf, failure, sev)
		switch sev {
		case lint.SeverityWarning:
			warningMap[failure.RuleName]++
			totalWarnings++
		case lint.SeverityError:
			errorMap[failure.RuleName]++
			totalErrors++
		}
	}

	f.printSummary(&buf, totalErrors, totalWarnings)
	f.printStatistics(&buf, color.RedString("Errors:"), errorMap)
	f.printStatistics(&buf, color.YellowString("Warnings:"), warningMap)
	return buf.String(), nil
}

func (f *Friendly) printFriendlyFailure(sb *strings.Builder, failure lint.Failure, severity lint.Severity) {
	f.printHeaderRow(sb, failure, severity)
	f.printFilePosition(sb, failure)
	sb.WriteString("\n\n")
}

func (f *Friendly) printHeaderRow(sb *strings.Builder, failure lint.Failure, severity lint.Severity) {
	emoji := getWarningEmoji()
	if severity == lint.SeverityError {
		emoji = getErrorEmoji()
	}
	sb.WriteString(f.table([][]string{{emoji, ruleDescriptionURL(failure.RuleName), color.GreenString(failure.Failure)}}))
}

func (*Friendly) printFilePosition(sb *strings.Builder, failure lint.Failure) {
	sb.WriteString(fmt.Sprintf("  %s:%d:%d", failure.GetFilename(), failure.Position.Start.Line, failure.Position.Start.Column))
}

type statEntry struct {
	name     string
	failures int
}

func (*Friendly) printSummary(w io.Writer, errors, warnings int) {
	emoji := getWarningEmoji()
	if errors > 0 {
		emoji = getErrorEmoji()
	}
	problemsLabel := "problems"
	if errors+warnings == 1 {
		problemsLabel = "problem"
	}
	warningsLabel := "warnings"
	if warnings == 1 {
		warningsLabel = "warning"
	}
	errorsLabel := "errors"
	if errors == 1 {
		errorsLabel = "error"
	}
	str := fmt.Sprintf("%d %s (%d %s, %d %s)", errors+warnings, problemsLabel, errors, errorsLabel, warnings, warningsLabel)
	if errors > 0 {
		fmt.Fprintf(w, "%s %s\n\n", emoji, color.RedString(str))
		return
	}
	if warnings > 0 {
		fmt.Fprintf(w, "%s %s\n\n", emoji, color.YellowString(str))
		return
	}
}

func (f *Friendly) printStatistics(w io.Writer, header string, stats map[string]int) {
	if len(stats) == 0 {
		return
	}
	data := make([]statEntry, 0, len(stats))
	for name, total := range stats {
		data = append(data, statEntry{name, total})
	}
	slices.SortFunc(data, func(a, b statEntry) int {
		return -cmp.Compare(a.failures, b.failures)
	})
	formatted := [][]string{}
	for _, entry := range data {
		formatted = append(formatted, []string{color.GreenString(fmt.Sprintf("%d", entry.failures)), entry.name})
	}
	fmt.Fprintln(w, header)
	fmt.Fprintln(w, f.table(formatted))
}

func (*Friendly) table(rows [][]string) string {
	buf := new(bytes.Buffer)
	table := tablewriter.NewWriter(buf)
	table.SetBorder(false)
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetAutoWrapText(false)
	table.AppendBulk(rows)
	table.Render()
	return buf.String()
}
