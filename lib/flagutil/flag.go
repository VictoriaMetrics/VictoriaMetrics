package flagutil

import (
	"flag"
	"fmt"
	"io"
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

func IsSet(flagName string) bool {
	isSet := false
	flag.Visit(func(f *flag.Flag) {
		if isSet {
			return
		}
		lname := strings.ToLower(f.Name)
		if flagName = strings.ToLower(flagName); lname == flagName {
			isSet = true
		}
	})
	return isSet
}
