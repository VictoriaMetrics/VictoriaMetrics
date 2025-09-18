package httputil

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// GetDuration returns duration in milliseconds from the given argKey query arg.
func GetDuration(r *http.Request, argKey string, defaultValue int64) (int64, error) {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return defaultValue, nil
	}
	if argValue == "undefined" {
		// This hack is needed for Grafana, which may send undefined value
		return defaultValue, nil
	}
	secs, err := strconv.ParseFloat(argValue, 64)
	if err != nil {
		// Try parsing string format
		d, err := timeutil.ParseDuration(argValue)
		if err != nil {
			return 0, fmt.Errorf("cannot parse %q=%q: %w", argKey, argValue, err)
		}
		secs = d.Seconds()
	}
	msecs := int64(secs * 1e3)
	if msecs <= 0 || msecs > maxDurationMsecs {
		return 0, fmt.Errorf("%s=%dms is out of allowed range [%dms ... %dms]", argKey, msecs, 1, int64(maxDurationMsecs))
	}
	return msecs, nil
}

const maxDurationMsecs = 100 * 365 * 24 * 3600 * 1000
