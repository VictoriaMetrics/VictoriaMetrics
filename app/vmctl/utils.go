package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

const barTpl = `{{ blue "%s:" }} {{ counters . }} {{ bar . "[" "█" (cycle . "█") "▒" "]" }} {{ percent . }}`

type timeRange struct {
	start time.Time
	end   time.Time
}

const (
	GranularityMonth string = "month"
	GranularityDay   string = "day"
	GranularityHour  string = "hour"
)

func prompt(question string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print(question, " [Y/n] ")
	answer, err := reader.ReadString('\n')
	if err != nil {
		panic(err)
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "" || answer == "yes" || answer == "y" {
		return true
	}
	return false
}

func wrapErr(vmErr *vm.ImportError, verbose bool) error {
	var errTS string
	var maxTS, minTS int64
	for _, ts := range vmErr.Batch {
		if minTS < ts.Timestamps[0] || minTS == 0 {
			minTS = ts.Timestamps[0]
		}
		if maxTS < ts.Timestamps[len(ts.Timestamps)-1] {
			maxTS = ts.Timestamps[len(ts.Timestamps)-1]
		}
		if verbose {
			errTS += fmt.Sprintf("%s for timestamps range %d - %d\n",
				ts.String(), ts.Timestamps[0], ts.Timestamps[len(ts.Timestamps)-1])
		}
	}
	var verboseMsg string
	if !verbose {
		verboseMsg = "(enable `--verbose` output to get more details)"
	}
	if vmErr.Err == nil {
		return fmt.Errorf("%s\n\tLatest delivered batch for timestamps range %d - %d %s\n%s",
			vmErr.Err, minTS, maxTS, verboseMsg, errTS)
	}
	return fmt.Errorf("%s\n\tImporting batch failed for timestamps range %d - %d %s\n%s",
		vmErr.Err, minTS, maxTS, verboseMsg, errTS)
}

type stepper interface {
	next(t time.Time) (time.Time, time.Time)
}

type monthStepper struct {
	generatedFirst bool
}

func (ms *monthStepper) next(t time.Time) (time.Time, time.Time) {
	if !ms.generatedFirst {
		ms.generatedFirst = true
		endOfCurrentMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location()).Add(-1 * time.Nanosecond)

		return t, endOfCurrentMonth
	}

	endOfNextMonth := time.Date(t.Year(), t.Month()+2, 1, 0, 0, 0, 0, t.Location()).Add(-1 * time.Nanosecond)
	startOfNextMonth := time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location())

	return startOfNextMonth, endOfNextMonth
}

type dayStepper struct{}

func (dayStepper) next(t time.Time) (time.Time, time.Time) {
	return t, t.AddDate(0, 0, 1)
}

type hourStepper struct{}

func (hourStepper) next(t time.Time) (time.Time, time.Time) {
	return t, t.Add(time.Hour * 1)
}

// splitDateRange splits range of dates in subset of ranges.
// Ranges with granularity of GranularityMonth are aligned to 1st of each month in order to improve export efficiency at block transfer level
func splitDateRange(start, end time.Time, granularity string) ([]timeRange, error) {

	if start.After(end) {
		return nil, fmt.Errorf("start time should be after end: start - %s, end - %s", start.Format(time.RFC3339), end.Format(time.RFC3339))
	}

	var st stepper

	switch granularity {
	case GranularityMonth:
		st = &monthStepper{}
	case GranularityDay:
		st = &dayStepper{}
	case GranularityHour:
		st = &hourStepper{}
	default:
		return nil, fmt.Errorf("failed to parse %s, valid values are: '%s', '%s', '%s'. provided: '%s'", GranularityMonth, GranularityDay, GranularityHour, vmNativeFilterChunk, granularity)
	}

	currentStep := start

	ranges := make([]timeRange, 0)

	for end.After(currentStep) {
		startOfStep, endOfStep := st.next(currentStep)
		if endOfStep.After(end) {
			endOfStep = end
		}
		ranges = append(ranges, timeRange{
			start: startOfStep,
			end:   endOfStep,
		})
		currentStep = endOfStep
	}

	return ranges, nil
}
