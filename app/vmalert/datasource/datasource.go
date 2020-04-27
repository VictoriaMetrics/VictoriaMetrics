package datasource

import "context"

// Querier interface wraps Query method which
// executes given query and returns list of Metrics
// as result
type Querier interface {
	Query(ctx context.Context, query string) ([]Metric, error)
}

// Metric is the basic entity which should be return by datasource
// It represents single data point with full list of labels
type Metric struct {
	Labels    []Label
	Timestamp int64
	Value     float64
}

// Label represents metric's label
type Label struct {
	Name  string
	Value string
}
