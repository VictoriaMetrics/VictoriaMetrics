package storagepacelimiter

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pacelimiter"
)

// Search limits the pace of search calls when there is at least a single in-flight assisted merge.
var Search = pacelimiter.New()

// BigMerges limits the pace for big merges when there is at least a single in-flight small merge.
var BigMerges = pacelimiter.New()
