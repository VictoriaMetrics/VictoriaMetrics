package buildinfo

import (
	"flag"
	"fmt"
	"os"
	"regexp"
)

var version = flag.Bool("version", false, "Show VictoriaMetrics version")

// Version must be set via -ldflags '-X'
var Version string

var shortVersionRe = regexp.MustCompile(`v\d+\.\d+\.\d+(?:-enterprise)?(?:-cluster)?`)

// ShortVersion returns a shortened version
func ShortVersion() string {
	return shortVersionRe.FindString(Version)
}

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
