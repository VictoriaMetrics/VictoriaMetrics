package flagutil

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// NewIntWithDynamicDefault returns a new int flag with the given name, defaultValue and description.
//
// Use it instead of flag.Int when defaultValue is calculated at runtime, for example
// from the number of CPU cores. Such a value differs per machine, so -help shows both
// the value and defaultValueHint, for example "16 = 2 * availableCPUs".
//
// Only -help output changes. The flag value stays defaultValue.
func NewIntWithDynamicDefault(name string, defaultValue int, defaultValueHint, description string) *int {
	if defaultValueHint == "" {
		panic(fmt.Sprintf("BUG: missing defaultValueHint for -%s", name))
	}
	p := flag.Int(name, defaultValue, description)

	// DefValue is only the text shown by -help: "default value (as text); for usage message".
	flag.Lookup(name).DefValue = fmt.Sprintf("%d = %s", defaultValue, defaultValueHint)

	return p
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
