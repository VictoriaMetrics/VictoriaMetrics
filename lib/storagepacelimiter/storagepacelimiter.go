package storagepacelimiter

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pacelimiter"
)

// Search limits the pace of search calls when there is at least a single in-flight assisted merge.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/291
var Search = pacelimiter.New()

// BigMerges limits the pace for big merges when there is at least a single in-flight small merge.
//
// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/648
var BigMerges = pacelimiter.New()
