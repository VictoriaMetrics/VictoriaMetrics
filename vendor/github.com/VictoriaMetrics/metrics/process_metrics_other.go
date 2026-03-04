//go:build !linux && !windows && !solaris && !darwin

package metrics

import (
	"io"
)

func writeProcessMetrics(w io.Writer) {
	// TODO: implement it
}

func writeFDMetrics(w io.Writer) {
	// TODO: implement it.
}
