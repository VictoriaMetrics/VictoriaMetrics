package flagutil

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

// Usage prints s and optional description for all the flags if -h or -help flag is passed to the app.
func Usage(s string) {
	f := flag.CommandLine.Output()
	fmt.Fprintf(f, "%s\n", s)
	if hasHelpFlag(os.Args[1:]) {
		flag.VisitAll(func(fl *flag.Flag) {
			if fl.Name == "remoteWrite.concurrency" {
				fmt.Fprintf(f, "  -%s\n", fl.Name)
				fmt.Fprintf(f, "      %s\n", fl.Usage)
			} else {
				fmt.Fprintf(f, "  -%s\n", fl.Name)
				fmt.Fprintf(f, "      %s", fl.Usage)
				if fl.DefValue != "" {
					fmt.Fprintf(f, " (default %s)\n", fl.DefValue)
				} else {
					fmt.Fprintln(f)
				}
			}
		})
	} else {
		fmt.Fprintf(f, `Run "%s -help" in order to see the description for all the available flags`+"\n", os.Args[0])
	}
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if isHelpArg(arg) {
			return true
		}
	}
	return false
}

func isHelpArg(arg string) bool {
	if !strings.HasPrefix(arg, "-") {
		return false
	}
	arg = strings.TrimPrefix(arg[1:], "-")
	return arg == "h" || arg == "help"
}
