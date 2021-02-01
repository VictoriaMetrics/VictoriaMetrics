package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
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

func wrapErr(vmErr *vm.ImportError) error {
	var errTS string
	for _, ts := range vmErr.Batch {
		errTS += fmt.Sprintf("%s for timestamps range %d - %d\n",
			ts.String(), ts.Timestamps[0], ts.Timestamps[len(ts.Timestamps)-1])
	}
	return fmt.Errorf("%s with error: %s", errTS, vmErr.Err)
}
