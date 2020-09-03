package storagepacelimiter

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pacelimiter"
)

// Search limits the pace of search calls when there is at least a single in-flight assisted merge.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/291
var Search = pacelimiter.New()
