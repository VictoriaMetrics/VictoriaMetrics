package native

import (
	"fmt"
	"sync"
	"time"
)

// Stats represents client statistic
// when processing data
type Stats struct {
	sync.Mutex
	StartTime    time.Time
	Bytes        uint64
	Requests     uint64
	Retries      uint64
}

func (s *Stats) String() string {
	s.Lock()
	defer s.Unlock()

	totalImportDuration := time.Since(s.StartTime)
	totalImportDurationS := totalImportDuration.Seconds()
	bytesPerS := byteCountSI(0)
	if s.Bytes > 0 && totalImportDurationS > 0 {
		bytesPerS = byteCountSI(int64(float64(s.Bytes) / totalImportDurationS))
	}

	return fmt.Sprintf("VictoriaMetrics importer stats:\n"+
		"  time spent while importing: %v;\n"+
		"  total bytes: %s;\n"+
		"  bytes/s: %s;\n"+
		"  import requests: %d;\n"+
		"  import requests retries: %d;",
		totalImportDuration,
		byteCountSI(int64(s.Bytes)), bytesPerS,
		s.Requests, s.Retries)
}

func byteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}
