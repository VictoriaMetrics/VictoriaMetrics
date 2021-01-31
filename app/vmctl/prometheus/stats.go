package prometheus

import (
	"fmt"
	"time"
)

// Stats represents data migration stats.
type Stats struct {
	Filtered      bool
	MinTime       int64
	MaxTime       int64
	Samples       uint64
	Series        uint64
	Blocks        int
	SkippedBlocks int
}

// String returns string representation for s.
func (s Stats) String() string {
	str := fmt.Sprintf("Prometheus snapshot stats:\n"+
		"  blocks found: %d;\n"+
		"  blocks skipped by time filter: %d;\n"+
		"  min time: %d (%v);\n"+
		"  max time: %d (%v);\n"+
		"  samples: %d;\n"+
		"  series: %d.",
		s.Blocks, s.SkippedBlocks,
		s.MinTime, time.Unix(s.MinTime/1e3, 0).Format(time.RFC3339),
		s.MaxTime, time.Unix(s.MaxTime/1e3, 0).Format(time.RFC3339),
		s.Samples, s.Series)

	if s.Filtered {
		str += "\n* Stats numbers are based on blocks meta info and don't account for applied filters."
	}

	return str
}
