package datasource

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// Querier interface wraps Query and QueryRange methods
type Querier interface {
	// Query executes instant request with the given query at the given ts.
	// It returns list of Metric in response, the http.Request used for sending query
	// and error if any. Returned http.Request can't be reused and its body is already read.
	// Query should stop once ctx is cancelled.
	Query(ctx context.Context, query string, ts time.Time) (Result, *http.Request, error)
	// QueryRange executes range request with the given query on the given time range.
	// It returns list of Metric in response and error if any.
	// QueryRange should stop once ctx is cancelled.
	QueryRange(ctx context.Context, query string, from, to time.Time) (Result, error)
}

// Result represents expected response from the datasource
type Result struct {
	// Data contains list of received Metric
	Data []Metric
	// SeriesFetched contains amount of time series processed by datasource
	// during query evaluation.
	// If nil, then this feature is not supported by the datasource.
	// SeriesFetched is supported by VictoriaMetrics since v1.90.
	SeriesFetched *int
	// IsPartial is used by VictoriaMetrics to indicate
	// whether response data is partial.
	IsPartial *bool
}

// QuerierBuilder builds Querier with given params.
type QuerierBuilder interface {
	// BuildWithParams creates a new Querier object with the given params
	BuildWithParams(params QuerierParams) Querier
}

// QuerierParams params for Querier.
type QuerierParams struct {
	DataSourceType string
	// ApplyIntervalAsTimeFilter is only valid for vlogs datasource.
	// Set to true if there is no [timeFilter](https://docs.victoriametrics.com/victorialogs/logsql/#time-filter) in the rule expression,
	// and we will add evaluation interval as an additional timeFilter when querying.
	ApplyIntervalAsTimeFilter bool
	EvaluationInterval        time.Duration
	QueryParams               url.Values
	Headers                   map[string]string
	Debug                     bool
}

// Metric is the basic entity which should be return by datasource
type Metric struct {
	Labels     []prompbmarshal.Label
	Timestamps []int64
	Values     []float64
}

// SetLabel adds or updates existing one label
// by the given key and label
func (m *Metric) SetLabel(key, value string) {
	for i, l := range m.Labels {
		if l.Name == key {
			m.Labels[i].Value = value
			return
		}
	}
	m.AddLabel(key, value)
}

// AddLabel appends the given label to the label set
func (m *Metric) AddLabel(key, value string) {
	m.Labels = append(m.Labels, prompbmarshal.Label{Name: key, Value: value})
}

// DelLabel deletes the given label from the label set
func (m *Metric) DelLabel(key string) {
	for i, l := range m.Labels {
		if l.Name == key {
			m.Labels = append(m.Labels[:i], m.Labels[i+1:]...)
		}
	}
}

// Label returns the given label value.
// If label is missing empty string will be returned
func (m *Metric) Label(key string) string {
	for _, l := range m.Labels {
		if l.Name == key {
			return l.Value
		}
	}
	return ""
}

// Labels is collection of Label
type Labels []prompbmarshal.Label

func (ls Labels) Len() int           { return len(ls) }
func (ls Labels) Swap(i, j int)      { ls[i], ls[j] = ls[j], ls[i] }
func (ls Labels) Less(i, j int) bool { return ls[i].Name < ls[j].Name }

func (ls Labels) String() string {
	var b bytes.Buffer

	b.WriteByte('{')
	for i, l := range ls {
		if i > 0 {
			b.WriteByte(',')
			b.WriteByte(' ')
		}
		b.WriteString(l.Name)
		b.WriteByte('=')
		b.WriteString(strconv.Quote(l.Value))
	}
	b.WriteByte('}')
	return b.String()
}

// LabelCompare return negative if a is less than b, return 0 if they are the same
// eg.
// a=[]Label{{Name: "a", Value: "1"}},b=[]Label{{Name: "b", Value: "1"}}, return -1
// a=[]Label{{Name: "a", Value: "2"}},b=[]Label{{Name: "a", Value: "1"}}, return 1
// a=[]Label{{Name: "a", Value: "1"}},b=[]Label{{Name: "a", Value: "1"}}, return 0
func LabelCompare(a, b Labels) int {
	l := min(len(b), len(a))

	for i := 0; i < l; i++ {
		if a[i].Name != b[i].Name {
			if a[i].Name < b[i].Name {
				return -1
			}
			return 1
		}
		if a[i].Value != b[i].Value {
			if a[i].Value < b[i].Value {
				return -1
			}
			return 1
		}
	}
	// if all labels so far were in common, the set with fewer labels comes first.
	return len(a) - len(b)
}

// ConvertToLabels convert map to Labels
func ConvertToLabels(m map[string]string) (labelset Labels) {
	for k, v := range m {
		labelset = append(labelset, prompbmarshal.Label{
			Name:  k,
			Value: v,
		})
	}
	// sort label
	sort.Slice(labelset, func(i, j int) bool { return labelset[i].Name < labelset[j].Name })
	return
}
