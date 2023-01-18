package flagutil

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

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

// SetFlagsFromEnvironment iterates over the defined flags using flag.VisitAll and
// attempts to set their value from the environment using flag.Set.
// For example, the function above sets -my-flag from the environment variable MY_FLAG if it exists.
func SetFlagsFromEnvironment() error {
	var err error
	flag.VisitAll(func(f *flag.Flag) {
		name := strings.ToUpper(strings.Replace(f.Name, "-", "_", -1))
		if value, ok := os.LookupEnv(name); ok {
			err = flag.Set(f.Name, value)
			if err != nil {
				err = fmt.Errorf("failed setting flag from environment: %w", err)
			}
		}
	})

	return err
}
