package config

import "time"

// Rule is basic alert entity
type Rule struct {
	Name        string
	Expr        string
	For         time.Duration
	Labels      map[string]string
	Annotations map[string]string
}

// Group grouping array of alert
type Group struct {
	Name  string
	Rules []Rule
}

// Parse parses config from given file
func Parse(filepath string) ([]Group, error) {
	return []Group{{
		Name: "foobar",
		Rules: []Rule{{
			Name: "vmrowsalert",
			Expr: "vm_rows",
			For:  1 * time.Second,
			Labels: map[string]string{
				"alert_label":  "value1",
				"alert_label2": "value2",
			},
			Annotations: map[string]string{
				"summary":     "{{ $value }}",
				"description": "LABELS: {{ $labels }}",
			},
		}},
	}}, nil
}
