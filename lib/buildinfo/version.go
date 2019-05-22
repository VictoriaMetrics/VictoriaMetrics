package buildinfo

import (
	"flag"
	"fmt"
	"os"
)

var version = flag.Bool("version", false, "Show VictoriaMetrics version")

// Version must be set via -ldflags '-X'
var Version string

// Init must be called after flag.Parse call.
func Init() {
	if *version {
		printVersion()
		os.Exit(0)
	}
}

func init() {
	oldUsage := flag.Usage
	flag.Usage = func() {
		printVersion()
		oldUsage()
	}
}

func printVersion() {
	fmt.Fprintf(flag.CommandLine.Output(), "%s\n", Version)
}
