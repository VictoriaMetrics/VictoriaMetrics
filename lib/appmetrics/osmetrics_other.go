//go:build !linux && !windows && !darwin

package appmetrics

import (
	"io"
)

func writeOSMetrics(w io.Writer) {
}
