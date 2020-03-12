package datasource

// Metric is the basic entity which should be return by datasource
// It represents single data point with full list of labels
type Metric struct {
	Labels    []Label
	Timestamp int64
	Value     float64
}

// Labels represents metric's label
type Label struct {
	Name  string
	Value string
}
