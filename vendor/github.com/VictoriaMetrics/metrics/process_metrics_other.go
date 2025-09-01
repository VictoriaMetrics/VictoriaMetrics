//go:build !linux && !windows && !solaris

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
