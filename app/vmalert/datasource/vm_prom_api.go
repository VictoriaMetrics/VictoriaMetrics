package datasource

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type promResponse struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	} `json:"data"`
}

type promInstant struct {
	Result []struct {
		Labels map[string]string `json:"metric"`
		TV     [2]interface{}    `json:"value"`
	} `json:"result"`
}

type promRange struct {
	Result []struct {
		Labels map[string]string `json:"metric"`
		TVs    [][2]interface{}  `json:"values"`
	} `json:"result"`
}

func (r promInstant) metrics() ([]Metric, error) {
	var result []Metric
	for i, res := range r.Result {
		f, err := strconv.ParseFloat(res.TV[1].(string), 64)
		if err != nil {
			return nil, fmt.Errorf("metric %v, unable to parse float64 from %s: %w", res, res.TV[1], err)
		}
		var m Metric
		for k, v := range r.Result[i].Labels {
			m.AddLabel(k, v)
		}
		m.Timestamps = append(m.Timestamps, int64(res.TV[0].(float64)))
		m.Values = append(m.Values, f)
		result = append(result, m)
	}
	return result, nil
}

func (r promRange) metrics() ([]Metric, error) {
	var result []Metric
	for i, res := range r.Result {
		var m Metric
		for _, tv := range res.TVs {
			f, err := strconv.ParseFloat(tv[1].(string), 64)
			if err != nil {
				return nil, fmt.Errorf("metric %v, unable to parse float64 from %s: %w", res, tv[1], err)
			}
			m.Values = append(m.Values, f)
			m.Timestamps = append(m.Timestamps, int64(tv[0].(float64)))
		}
		if len(m.Values) < 1 || len(m.Timestamps) < 1 {
			return nil, fmt.Errorf("metric %v contains no values", res)
		}
		m.Labels = nil
		for k, v := range r.Result[i].Labels {
			m.AddLabel(k, v)
		}
		result = append(result, m)
	}
	return result, nil
}

const (
	statusSuccess, statusError = "success", "error"
	rtVector, rtMatrix         = "vector", "matrix"
)

func parsePrometheusResponse(req *http.Request, resp *http.Response) ([]Metric, error) {
	r := &promResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, fmt.Errorf("error parsing prometheus metrics for %s: %w", req.URL.Redacted(), err)
	}
	if r.Status == statusError {
		return nil, fmt.Errorf("response error, query: %s, errorType: %s, error: %s", req.URL.Redacted(), r.ErrorType, r.Error)
	}
	if r.Status != statusSuccess {
		return nil, fmt.Errorf("unknown status: %s, Expected success or error ", r.Status)
	}
	switch r.Data.ResultType {
	case rtVector:
		var pi promInstant
		if err := json.Unmarshal(r.Data.Result, &pi.Result); err != nil {
			return nil, fmt.Errorf("umarshal err %s; \n %#v", err, string(r.Data.Result))
		}
		return pi.metrics()
	case rtMatrix:
		var pr promRange
		if err := json.Unmarshal(r.Data.Result, &pr.Result); err != nil {
			return nil, err
		}
		return pr.metrics()
	default:
		return nil, fmt.Errorf("unknown result type %q", r.Data.ResultType)
	}
}

const (
	prometheusInstantPath = "/api/v1/query"
	prometheusRangePath   = "/api/v1/query_range"
	prometheusPrefix      = "/prometheus"
)

func (s *VMStorage) setPrometheusInstantReqParams(r *http.Request, query string, timestamp time.Time) {
	if s.appendTypePrefix {
		r.URL.Path += prometheusPrefix
	}
	if !s.disablePathAppend {
		r.URL.Path += prometheusInstantPath
	}
	q := r.URL.Query()
	if s.lookBack > 0 {
		timestamp = timestamp.Add(-s.lookBack)
	}
	if s.evaluationInterval > 0 {
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1232
		timestamp = timestamp.Truncate(s.evaluationInterval)
	}
	q.Set("time", fmt.Sprintf("%d", timestamp.Unix()))
	r.URL.RawQuery = q.Encode()
	s.setPrometheusReqParams(r, query)
}

func (s *VMStorage) setPrometheusRangeReqParams(r *http.Request, query string, start, end time.Time) {
	if s.appendTypePrefix {
		r.URL.Path += prometheusPrefix
	}
	if !s.disablePathAppend {
		r.URL.Path += prometheusRangePath
	}
	q := r.URL.Query()
	q.Add("start", fmt.Sprintf("%d", start.Unix()))
	q.Add("end", fmt.Sprintf("%d", end.Unix()))
	r.URL.RawQuery = q.Encode()
	s.setPrometheusReqParams(r, query)
}

func (s *VMStorage) setPrometheusReqParams(r *http.Request, query string) {
	q := r.URL.Query()
	for k, vs := range s.extraParams {
		if q.Has(k) { // extraParams are prior to params in URL
			q.Del(k)
		}
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("query", query)
	if s.evaluationInterval > 0 { // set step as evaluationInterval by default
		// always convert to seconds to keep compatibility with older
		// Prometheus versions. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1943
		q.Set("step", fmt.Sprintf("%ds", int(s.evaluationInterval.Seconds())))
	}
	if s.queryStep > 0 { // override step with user-specified value
		// always convert to seconds to keep compatibility with older
		// Prometheus versions. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1943
		q.Set("step", fmt.Sprintf("%ds", int(s.queryStep.Seconds())))
	}
	r.URL.RawQuery = q.Encode()
}
