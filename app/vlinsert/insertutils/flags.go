package insertutils

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

var (
	// MaxLineSizeBytes is the maximum length of a single line for /insert/* handlers
	MaxLineSizeBytes = flagutil.NewBytes("insert.maxLineSizeBytes", 256*1024, "The maximum size of a single line, which can be read by /insert/* handlers")
)
