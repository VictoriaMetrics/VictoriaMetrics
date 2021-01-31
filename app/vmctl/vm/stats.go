package vm

import (
	"fmt"
	"sync"
	"time"
)

type stats struct {
	sync.Mutex
	samples      uint64
	bytes        uint64
	requests     uint64
	retries      uint64
	startTime    time.Time
	idleDuration time.Duration
}

func (s *stats) String() string {
	s.Lock()
	defer s.Unlock()

	totalImportDuration := time.Since(s.startTime)
	totalImportDurationS := totalImportDuration.Seconds()
	var samplesPerS float64
	if s.samples > 0 && totalImportDurationS > 0 {
		samplesPerS = float64(s.samples) / totalImportDurationS
	}
	bytesPerS := byteCountSI(0)
	if s.bytes > 0 && totalImportDurationS > 0 {
		bytesPerS = byteCountSI(int64(float64(s.bytes) / totalImportDurationS))
	}

	return fmt.Sprintf("VictoriaMetrics importer stats:\n"+
		"  idle duration: %v;\n"+
		"  time spent while importing: %v;\n"+
		"  total samples: %d;\n"+
		"  samples/s: %.2f;\n"+
		"  total bytes: %s;\n"+
		"  bytes/s: %s;\n"+
		"  import requests: %d;\n"+
		"  import requests retries: %d;",
		s.idleDuration, totalImportDuration,
		s.samples, samplesPerS,
		byteCountSI(int64(s.bytes)), bytesPerS,
		s.requests, s.retries)
}
