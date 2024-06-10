package datasource

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/valyala/fastjson"
)

var (
	disablePathAppend = flag.Bool("remoteRead.disablePathAppend", false, "Whether to disable automatic appending of '/api/v1/query' path "+
		"to the configured -datasource.url and -remoteRead.url")
	disableStepParam = flag.Bool("datasource.disableStepParam", false, "Whether to disable adding 'step' param to the issued instant queries. "+
		"This might be useful when using vmalert with datasources that do not support 'step' param for instant queries, like Google Managed Prometheus. "+
		"It is not recommended to enable this flag if you use vmalert with VictoriaMetrics.")
)

type promResponse struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
	Data      struct {
		ResultType string          `json:"resultType"`
		Result     json.RawMessage `json:"result"`
	} `json:"data"`
	// Stats supported by VictoriaMetrics since v1.90
	Stats struct {
		SeriesFetched *string `json:"seriesFetched,omitempty"`
	} `json:"stats,omitempty"`
}

// see https://prometheus.io/docs/prometheus/latest/querying/api/#instant-queries
type promInstant struct {
	// ms is populated after Unmarshal call
	ms []Metric
}

// metrics returned parsed Metric slice
// Must be called only after Unmarshal
func (pi *promInstant) metrics() ([]Metric, error) {
	return pi.ms, nil
}

var jsonParserPool fastjson.ParserPool

// Unmarshal unmarshals the given byte slice into promInstant
// It is using fastjson to reduce number of allocations compared to
// standard json.Unmarshal function.
// Response example:
//
//	[{"metric":{"__name__":"up","job":"prometheus"},value": [ 1435781451.781,"1"]},
//	{"metric":{"__name__":"up","job":"node"},value": [ 1435781451.781,"0"]}]
func (pi *promInstant) Unmarshal(b []byte) error {
	p := jsonParserPool.Get()
	defer jsonParserPool.Put(p)

	v, err := p.ParseBytes(b)
	if err != nil {
		return err
	}

	rows, err := v.Array()
	if err != nil {
		return fmt.Errorf("cannot find the top-level array of result objects: %w", err)
	}
	pi.ms = make([]Metric, len(rows))
	for i, row := range rows {
		metric := row.Get("metric")
		if metric == nil {
			return fmt.Errorf("can't find `metric` object in %q", row)
		}
		labels := metric.GetObject()

		r := &pi.ms[i]
		r.Labels = make([]Label, 0, labels.Len())
		labels.Visit(func(key []byte, v *fastjson.Value) {
			lv, errLocal := v.StringBytes()
			if errLocal != nil {
				err = fmt.Errorf("error when parsing label value %q: %s", v, errLocal)
				return
			}
			r.Labels = append(r.Labels, Label{
				Name:  string(key),
				Value: string(lv),
			})
		})
		if err != nil {
			return fmt.Errorf("error when parsing `metric` object in %q: %w", row, err)
		}

		value := row.Get("value")
		if value == nil {
			return fmt.Errorf("can't find `value` object in %q", row)
		}
		sample := value.GetArray()
		if len(sample) != 2 {
			return fmt.Errorf("object `value` in %q should contain 2 values, but contains %d instead", row, len(sample))
		}
		r.Timestamps = []int64{sample[0].GetInt64()}
		val, err := sample[1].StringBytes()
		if err != nil {
			return fmt.Errorf("error when parsing `value` object %q: %s", sample[1], err)
		}
		f, err := strconv.ParseFloat(bytesutil.ToUnsafeString(val), 64)
		if err != nil {
			return fmt.Errorf("error when parsing float64 from %s in %q: %w", sample[1], row, err)
		}
		r.Values = []float64{f}
	}
	return nil
}

type promRange struct {
	Result []struct {
		Labels map[string]string `json:"metric"`
		TVs    [][2]interface{}  `json:"values"`
	} `json:"result"`
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

type promScalar [2]interface{}

func (r promScalar) metrics() ([]Metric, error) {
	var m Metric
	f, err := strconv.ParseFloat(r[1].(string), 64)
	if err != nil {
		return nil, fmt.Errorf("metric %v, unable to parse float64 from %s: %w", r, r[1], err)
	}
	m.Values = append(m.Values, f)
	m.Timestamps = append(m.Timestamps, int64(r[0].(float64)))
	return []Metric{m}, nil
}

const (
	statusSuccess, statusError  = "success", "error"
	rtVector, rtMatrix, rScalar = "vector", "matrix", "scalar"
)

func parsePrometheusResponse(req *http.Request, resp *http.Response) (res Result, err error) {
	r := &promResponse{}
	if err = json.NewDecoder(resp.Body).Decode(r); err != nil {
		return res, fmt.Errorf("error parsing prometheus metrics for %s: %w", req.URL.Redacted(), err)
	}
	if r.Status == statusError {
		return res, fmt.Errorf("response error, query: %s, errorType: %s, error: %s", req.URL.Redacted(), r.ErrorType, r.Error)
	}
	if r.Status != statusSuccess {
		return res, fmt.Errorf("unknown status: %s, Expected success or error", r.Status)
	}
	var parseFn func() ([]Metric, error)
	switch r.Data.ResultType {
	case rtVector:
		var pi promInstant
		if err := pi.Unmarshal(r.Data.Result); err != nil {
			return res, fmt.Errorf("unmarshal err %w; \n %#v", err, string(r.Data.Result))
		}
		parseFn = pi.metrics
	case rtMatrix:
		var pr promRange
		if err := json.Unmarshal(r.Data.Result, &pr.Result); err != nil {
			return res, err
		}
		parseFn = pr.metrics
	case rScalar:
		var ps promScalar
		if err := json.Unmarshal(r.Data.Result, &ps); err != nil {
			return res, err
		}
		parseFn = ps.metrics
	default:
		return res, fmt.Errorf("unknown result type %q", r.Data.ResultType)
	}

	ms, err := parseFn()
	if err != nil {
		return res, err
	}
	res = Result{Data: ms}
	if r.Stats.SeriesFetched != nil {
		intV, err := strconv.Atoi(*r.Stats.SeriesFetched)
		if err != nil {
			return res, fmt.Errorf("failed to convert stats.seriesFetched to int: %w", err)
		}
		res.SeriesFetched = &intV
	}
	return res, nil
}

func (s *VMStorage) setPrometheusInstantReqParams(r *http.Request, query string, timestamp time.Time) {
	if s.appendTypePrefix {
		r.URL.Path += "/prometheus"
	}
	if !*disablePathAppend {
		r.URL.Path += "/api/v1/query"
	}
	q := r.URL.Query()
	q.Set("time", timestamp.Format(time.RFC3339))
	if !*disableStepParam && s.evaluationInterval > 0 { // set step as evaluationInterval by default
		// always convert to seconds to keep compatibility with older
		// Prometheus versions. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1943
		q.Set("step", fmt.Sprintf("%ds", int(s.evaluationInterval.Seconds())))
	}
	if !*disableStepParam && s.queryStep > 0 { // override step with user-specified value
		// always convert to seconds to keep compatibility with older
		// Prometheus versions. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1943
		q.Set("step", fmt.Sprintf("%ds", int(s.queryStep.Seconds())))
	}
	r.URL.RawQuery = q.Encode()
	s.setPrometheusReqParams(r, query)
}

func (s *VMStorage) setPrometheusRangeReqParams(r *http.Request, query string, start, end time.Time) {
	if s.appendTypePrefix {
		r.URL.Path += "/prometheus"
	}
	if !*disablePathAppend {
		r.URL.Path += "/api/v1/query_range"
	}
	q := r.URL.Query()
	q.Add("start", start.Format(time.RFC3339))
	q.Add("end", end.Format(time.RFC3339))
	if s.evaluationInterval > 0 { // set step as evaluationInterval by default
		// always convert to seconds to keep compatibility with older
		// Prometheus versions. See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1943
		q.Set("step", fmt.Sprintf("%ds", int(s.evaluationInterval.Seconds())))
	}
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
	r.URL.RawQuery = q.Encode()
}
