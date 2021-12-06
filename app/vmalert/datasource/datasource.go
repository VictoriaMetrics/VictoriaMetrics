package datasource

import (
	"context"
	"net/url"
	"time"
)

// Querier interface wraps Query and QueryRange methods
type Querier interface {
	Query(ctx context.Context, query string) ([]Metric, error)
	QueryRange(ctx context.Context, query string, from, to time.Time) ([]Metric, error)
}

// QuerierBuilder builds Querier with given params.
type QuerierBuilder interface {
	BuildWithParams(params QuerierParams) Querier
}

// QuerierParams params for Querier.
type QuerierParams struct {
	DataSourceType     *Type
	EvaluationInterval time.Duration
	QueryParams        url.Values
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

// AddLabel appends the given label to the label set
func (m *Metric) AddLabel(key, value string) {
	m.Labels = append(m.Labels, Label{Name: key, Value: value})
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
