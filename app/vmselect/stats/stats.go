package stats

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/searchutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// MetricNamesStatsHandler returns timeseries metric names usage statistics
func MetricNamesStatsHandler(startTime time.Time, at *auth.Token, qt *querytracer.Tracer, w http.ResponseWriter, r *http.Request) error {
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
	deadline := searchutil.GetDeadlineForStatusRequest(r, startTime)
	var tt *storage.TenantToken
	if at != nil {
		tt = &storage.TenantToken{
			AccountID: at.AccountID,
			ProjectID: at.ProjectID,
		}
	}
	matchNames := r.Form["match_names"]
	statsQuery := storage.MetricNamesStatsQuery{
		TenantToken:  tt,
		Limit:        limit,
		Le:           le,
		MatchNames:   matchNames,
		MatchPattern: matchPattern,
	}
	if limit > 0 && len(matchNames) > limit {
		return fmt.Errorf("match_names len=%d cannot exceed limit=%d", len(matchNames), limit)
	}
	stats, err := netstorage.GetMetricNamesStats(qt, statsQuery, deadline)
	if err != nil {
		return err
	}
	WriteMetricNamesStatsResponse(w, &stats, qt)
	return nil
}

// ResetMetricNamesStatsHandler resets metric names usage state
func ResetMetricNamesStatsHandler(startTime time.Time, qt *querytracer.Tracer, r *http.Request) error {
	deadline := searchutil.GetDeadlineForStatusRequest(r, startTime)
	if err := netstorage.ResetMetricNamesStats(qt, deadline); err != nil {
		return err
	}
	return nil
}
