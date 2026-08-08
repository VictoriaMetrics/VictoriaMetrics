package flagutil

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// SetDynamicDefault sets the text shown as the default value for the given flag
// in `-help` output.
//
// It must be called for flags whose default value is calculated at runtime, for example
// from the number of available CPU cores. Such flags print the value calculated on the
// machine which runs the binary, so `-help` output and the docs generated from it claim
// a constant default which doesn't hold anywhere else.
//
// s should describe how the default value is calculated, for example "2 * availableCPUs".
func SetDynamicDefault(flagName, s string) {
	f := flag.Lookup(flagName)
	if f == nil {
		panic(fmt.Sprintf("BUG: unknown flag %s", flagName))
	}
	f.DefValue = s
}

// WriteFlags writes all the explicitly set flags to w.
func WriteFlags(w io.Writer) {
	flag.Visit(func(f *flag.Flag) {
		lname := strings.ToLower(f.Name)
		value := f.Value.String()
		if IsSecretFlag(lname) {
			value = "secret"
		}
		fmt.Fprintf(w, "-%s=%q\n", f.Name, value)
	})
}
