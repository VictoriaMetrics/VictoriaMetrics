package flagutil

import (
	"flag"
	"fmt"
	"io"
)

// WriteFlags writes all the explicitly set flags to w.
func WriteFlags(w io.Writer) {
	flag.Visit(func(f *flag.Flag) {
		value := f.Value.String()
		fmt.Fprintf(w, "-%s=%q\n", f.Name, value)
	})
}
