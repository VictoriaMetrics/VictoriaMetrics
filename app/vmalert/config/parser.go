package config

import "time"

// Labels basic struct of different labels
type Labels struct {
	Severity string
}

// Annotations basic annotation for alert rule
type Annotations struct {
	Summary     string
	Description string
}

// Alert basic alert entity rule
type Alert struct {
	Name        string
	Expr        string
	For         time.Duration
	Labels      Labels
	Annotations Annotations
}

// Group grouping array of alert
type Group struct {
	Name  string
	Rules []Alert
}

// Parse parses config from given file
func Parse(filepath string) ([]Group, error) {
	return []Group{}, nil
}
