package common

import "github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"

var (
	MaxLineSizeBytes = flagutil.NewBytes("insert.maxLineSizeBytes", 256*1024, "The maximum size of a single line, which can be read by /insert/* handlers")
)
