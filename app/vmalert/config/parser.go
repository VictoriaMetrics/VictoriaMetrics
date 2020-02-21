package config

import "time"

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
	Labels      map[string]string
	Annotations Annotations

	Start time.Time
	End   time.Time
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
