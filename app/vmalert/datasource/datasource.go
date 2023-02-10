package datasource

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Querier interface wraps Query and QueryRange methods
type Querier interface {
	// Query executes instant request with the given query at the given ts.
	// It returns list of Metric in response, the http.Request used for sending query
	// and error if any. Returned http.Request can't be reused and its body is already read.
	// Query should stop once ctx is cancelled.
	Query(ctx context.Context, query string, ts time.Time) ([]Metric, *http.Request, error)
	// QueryRange executes range request with the given query on the given time range.
	// It returns list of Metric in response and error if any.
	// QueryRange should stop once ctx is cancelled.
	QueryRange(ctx context.Context, query string, from, to time.Time) ([]Metric, error)
}

// QuerierBuilder builds Querier with given params.
type QuerierBuilder interface {
	// BuildWithParams creates a new Querier object with the given params
	BuildWithParams(params QuerierParams) Querier
}

// QuerierParams params for Querier.
type QuerierParams struct {
	DataSourceType     string
	EvaluationInterval time.Duration
	QueryParams        url.Values
	Headers            map[string]string
	Debug              bool
}

// Metric is the basic entity which should be return by datasource
type Metric struct {
	Labels     []Label
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

// SetLabels sets the given map as Metric labels
func (m *Metric) SetLabels(ls map[string]string) {
	var i int
	m.Labels = make([]Label, len(ls))
	for k, v := range ls {
		m.Labels[i] = Label{
			Name:  k,
			Value: v,
		}
		i++
	}
}

// AddLabel appends the given label to the label set
func (m *Metric) AddLabel(key, value string) {
	m.Labels = append(m.Labels, Label{Name: key, Value: value})
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

// Label represents metric's label
type Label struct {
	Name  string
	Value string
}
