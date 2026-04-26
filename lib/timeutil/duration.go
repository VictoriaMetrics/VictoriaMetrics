package timeutil

import (
	"fmt"
	"math"
	"time"

	"github.com/VictoriaMetrics/metricsql"
)

// ParseDuration parses duration string in Prometheus format
func ParseDuration(s string) (time.Duration, error) {
	ms, err := metricsql.DurationValue(s, 0)
	if err != nil {
		return 0, err
	}
	const maxMs = math.MaxInt64 / int64(time.Millisecond)
	if ms > maxMs || ms < -maxMs {
		maxD := time.Duration(maxMs) * time.Millisecond
		return 0, fmt.Errorf("duration %q must be in the range [%v, %v]", s, -maxD, maxD)
	}
	return time.Duration(ms) * time.Millisecond, nil
}
