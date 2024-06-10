package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/terminal"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

const barTpl = `{{ blue "%s:" }} {{ counters . }} {{ bar . "[" "█" (cycle . "█") "▒" "]" }} {{ percent . }}`

// isSilent should be inited in main
var isSilent bool

func prompt(question string) bool {
	if isSilent {
		return true
	}
	isTerminal := terminal.IsTerminal(int(os.Stdout.Fd()))
	if !isTerminal {
		return true
	}
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
