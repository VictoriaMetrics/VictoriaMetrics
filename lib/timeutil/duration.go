package timeutil

import (
	"fmt"
	"time"

	"github.com/VictoriaMetrics/metricsql"
)

var (
	minDuration = time.Duration(minValidMilli * time.Millisecond)
	maxDuration = time.Duration(maxValidMilli * time.Millisecond)
)

// ParseDuration parses duration string in Prometheus format
func ParseDuration(s string) (time.Duration, error) {
	ms, err := metricsql.DurationValue(s, 0)
	if err != nil {
		return 0, err
	}
	if ms < minValidMilli || maxValidMilli < ms {
		return 0, fmt.Errorf("duration %q must be in the range [%s, %s]", s, minDuration, maxDuration)
	}
	return time.Duration(ms) * time.Millisecond, nil
}
