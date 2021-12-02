package datasource

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type graphiteResponse []graphiteResponseTarget

type graphiteResponseTarget struct {
	Target     string            `json:"target"`
	Tags       map[string]string `json:"tags"`
	DataPoints [][2]float64      `json:"datapoints"`
}

func (r graphiteResponse) metrics() []Metric {
	var ms []Metric
	for _, res := range r {
		if len(res.DataPoints) < 1 {
			continue
		}
		var m Metric
		// add only last value to the result.
		last := res.DataPoints[len(res.DataPoints)-1]
		m.Values = append(m.Values, last[0])
		m.Timestamps = append(m.Timestamps, int64(last[1]))
		for k, v := range res.Tags {
			m.AddLabel(k, v)
		}
		ms = append(ms, m)
	}
	return ms
}

func parseGraphiteResponse(req *http.Request, resp *http.Response) ([]Metric, error) {
	r := &graphiteResponse{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, fmt.Errorf("error parsing graphite metrics for %s: %w", req.URL.Redacted(), err)
	}
	return r.metrics(), nil
}

const (
	graphitePath   = "/render"
	graphitePrefix = "/graphite"
)

func (s *VMStorage) setGraphiteReqParams(r *http.Request, query string, timestamp time.Time) {
	if s.appendTypePrefix {
		r.URL.Path += graphitePrefix
	}
	r.URL.Path += graphitePath
	q := r.URL.Query()
	for k, vs := range s.extraParams {
		if q.Has(k) { // extraParams are prior to params in URL
			q.Del(k)
		}
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("format", "json")
	q.Set("target", query)
	from := "-5min"
	if s.lookBack > 0 {
		lookBack := timestamp.Add(-s.lookBack)
		from = strconv.FormatInt(lookBack.Unix(), 10)
	}
	q.Set("from", from)
	q.Set("until", "now")
	r.URL.RawQuery = q.Encode()
}
