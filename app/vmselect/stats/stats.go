package stats

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
)

// MetricNamesStatsHandler returns timeseries metric names usage statistics
func MetricNamesStatsHandler(qt *querytracer.Tracer, w http.ResponseWriter, r *http.Request) error {
	limit := 1000
	limitStr := r.FormValue("limit")
	if len(limitStr) > 0 {
		n, err := strconv.Atoi(limitStr)
		if err != nil {
			return fmt.Errorf("cannot parse `limit` arg %q: %w", limitStr, err)
		}
		if n > 0 {
			limit = n
		}
	}
	// by default display all values
	le := -1
	leStr := r.FormValue("le")
	if len(leStr) > 0 {
		n, err := strconv.Atoi(leStr)
		if err != nil {
			return fmt.Errorf("cannot parse `le` arg %q: %w", leStr, err)
		}
		le = n
	}
	matchPattern := r.FormValue("match_pattern")
	stats, err := netstorage.GetMetricNamesStats(qt, limit, le, matchPattern)
	if err != nil {
		return err
	}
	WriteMetricNamesStatsResponse(w, &stats, qt)
	return nil
}

// ResetMetricNamesStatsHandler resets metric names usage state
func ResetMetricNamesStatsHandler(qt *querytracer.Tracer) error {
	if err := netstorage.ResetMetricNamesStats(qt); err != nil {
		return err
	}
	return nil
}
