package envflag

import (
	"flag"
	"os"
)

// Parse parses environment vars and command-line flags.
//
// Flags set via command-line override flags set via environment vars.
//
// This function must be called instead of flag.Parse() before using any flags in the program.
func Parse() {
	flag.Parse()

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
		if v, ok := os.LookupEnv(f.Name); ok {
			f.Value.Set(v)
		}
	})
}
