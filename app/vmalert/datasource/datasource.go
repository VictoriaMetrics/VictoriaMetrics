package datasource

// Metric represents single metric
type Metric struct {
	Label     []Label
	Timestamp int64
	Value     float64
}

// Label represents metric's label
type Label struct {
	Name  string
	Value string
}
