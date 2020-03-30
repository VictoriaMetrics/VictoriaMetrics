package envflag

import (
	"flag"
	"log"
	"os"
	"strings"
)

var (
	enable = flag.Bool("envflag.enable", false, "Whether to enable reading flags from environment variables additionally to command line. "+
		"Command line flag values have priority over values from environment vars. "+
		"Flags are read only from command line if this flag isn't set")
	prefix = flag.String("envflag.prefix", "", "Prefix for environment variables if -envflag.enable is set")
)

// Parse parses environment vars and command-line flags.
//
// Flags set via command-line override flags set via environment vars.
//
// This function must be called instead of flag.Parse() before using any flags in the program.
func Parse() {
	flag.Parse()
	if !*enable {
		return
	}

	// Remember explicitly set command-line flags.
	flagsSet := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		flagsSet[f.Name] = true
	})

	// Obtain the remaining flag values from environment vars.
	flag.VisitAll(func(f *flag.Flag) {
		if flagsSet[f.Name] {
			// The flag is explicitly set via command-line.
			return
		}
		// Get flag value from environment var.
		fname := getEnvFlagName(f.Name)
		if v, ok := os.LookupEnv(fname); ok {
			if err := f.Value.Set(v); err != nil {
				// Do not use lib/logger here, since it is uninitialized yet.
				log.Fatalf("cannot set flag %s to %q, which is read from environment variable %q: %s", f.Name, v, fname, err)
			}
		}
	})
}

func getEnvFlagName(s string) string {
	// Substitute dots with underscores, since env var names cannot contain dots.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/311#issuecomment-586354129 for details.
	s = strings.ReplaceAll(s, ".", "_")
	return *prefix + s
}
