package datasource

// Metric represents single metric
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
