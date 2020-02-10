package envflag

import (
	"flag"
	"log"
	"os"
)

var enable = flag.Bool("envflag.enable", false, "Whether to enable reading flags from environment variables additionally to command line. "+
	"Command line flag values have priority over values from envoronment vars. "+
	"Flags are read only from command line if this flag isn't set")

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
		if v, ok := os.LookupEnv(f.Name); ok {
			if err := f.Value.Set(v); err != nil {
				// Do not use lib/logger here, since it is uninitialized yet.
				log.Fatalf("cannot set flag %s to %q, which is read from environment variable: %s", f.Name, v, err)
			}
		}
	})
}
