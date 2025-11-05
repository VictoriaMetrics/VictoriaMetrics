package envflag

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envtemplate"
)

var (
	enable = flag.Bool("envflag.enable", false, "Whether to enable reading flags from environment variables in addition to the command line. "+
		"Command line flag values have priority over values from environment vars. "+
		"Flags are read only from the command line if this flag isn't set. See https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#environment-variables for more details")
	prefix = flag.String("envflag.prefix", "", "Prefix for environment variables if -envflag.enable is set")
)

// Parse parses environment vars and command-line flags.
//
// Flags set via command-line override flags set via environment vars.
//
// This function must be called instead of flag.Parse() before using any flags in the program.
func Parse() {
	ParseFlagSet(flag.CommandLine, os.Args[1:])
}

// ParseFlagSet parses the given args into the given fs.
func ParseFlagSet(fs *flag.FlagSet, args []string) {
	args = expandArgs(args)
	if err := fs.Parse(args); err != nil {
		// Do not use lib/logger here, since it is uninitialized yet.
		log.Fatalf("cannot parse flags %q: %s", args, err)
	}
	if fs.NArg() > 0 {
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4845
		log.Fatalf("unprocessed command-line args left: %s; the most likely reason is missing `=` between boolean flag name and value; "+
			"see https://pkg.go.dev/flag#hdr-Command_line_flag_syntax", fs.Args())
	}
	if !*enable {
		return
	}
	// Remember explicitly set command-line flags.
	flagsSet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		flagsSet[f.Name] = true
	})

	// Obtain the remaining flag values from environment vars.
	fs.VisitAll(func(f *flag.Flag) {
		if flagsSet[f.Name] {
			// The flag is explicitly set via command-line.
			return
		}
		// Get flag value from environment var.
		fname := getEnvFlagName(f.Name)
		if v, ok := envtemplate.LookupEnv(fname); ok {
			if err := fs.Set(f.Name, v); err != nil {
				// Do not use lib/logger here, since it is uninitialized yet.
				log.Fatalf("cannot set flag %s to %q, which is read from env var %q: %s", f.Name, v, fname, err)
			}
		}
	})
}

// expandArgs substitutes %{ENV_VAR} placeholders inside args
// with the corresponding environment variable values.
func expandArgs(args []string) []string {
	dstArgs := make([]string, 0, len(args))
	for _, arg := range args {
		s := envtemplate.ReplaceString(arg)
		if len(s) > 0 {
			dstArgs = append(dstArgs, s)
		}
	}
	return dstArgs
}

func getEnvFlagName(s string) string {
	// Substitute dots with underscores, since env var names cannot contain dots.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/311#issuecomment-586354129 for details.
	s = strings.ReplaceAll(s, ".", "_")
	return *prefix + s
}
