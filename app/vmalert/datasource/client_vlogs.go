package datasource

import (
	"fmt"
	"net/http"
	"time"
)

func (s *Client) setVLogsInstantReqParams(r *http.Request, query string, timestamp time.Time) {
	// there is no type path prefix in victorialogs APIs right now, ignore appendTypePrefix.
	if !*disablePathAppend {
		r.URL.Path += "/select/logsql/stats_query"
	}
	q := r.URL.Query()
	q.Set("time", timestamp.Format(time.RFC3339))
	if *applyIntervalAsTimeFilter && s.evaluationInterval > 0 {
		q.Set("start", timestamp.Add(-s.evaluationInterval).Format(time.RFC3339))
		q.Set("end", timestamp.Format(time.RFC3339))
	}
	r.URL.RawQuery = q.Encode()
	s.setReqParams(r, query)
}

func (s *Client) setVLogsRangeReqParams(r *http.Request, query string, start, end time.Time) {
	// there is no type path prefix in victorialogs APIs right now, ignore appendTypePrefix.
	if !*disablePathAppend {
		r.URL.Path += "/select/logsql/stats_query_range"
	}
	q := r.URL.Query()
	q.Add("start", start.Format(time.RFC3339))
	q.Add("end", end.Format(time.RFC3339))
	// set step as evaluationInterval by default
	if s.evaluationInterval > 0 {
		q.Set("step", fmt.Sprintf("%ds", int(s.evaluationInterval.Seconds())))
	}
	r.URL.RawQuery = q.Encode()
	s.setReqParams(r, query)
}

func parseVLogsResponse(req *http.Request, resp *http.Response) (res Result, err error) {
	res, err = parsePrometheusResponse(req, resp)
	if err != nil {
		return Result{}, err
	}
	for i := range res.Data {
		m := &res.Data[i]
		for j := range m.Labels {
			// reserve the stats func result name with a new label `stats_result` instead of dropping it,
			// since there could be multiple stats results in a single query, for instance:
			// 	_time:5m | stats quantile(0.5, request_duration_seconds) p50, quantile(0.9, request_duration_seconds) p90
			if m.Labels[j].Name == "__name__" {
				m.Labels[j].Name = "stats_result"
				break
			}
		}
	}
	return
}
