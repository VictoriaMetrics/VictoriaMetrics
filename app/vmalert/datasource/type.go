package datasource

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/graphiteql"
	"github.com/VictoriaMetrics/metricsql"
)

// Type represents data source type
type Type struct {
	name string
}

// NewPrometheusType returns prometheus datasource type
func NewPrometheusType() Type {
	return Type{
		name: "prometheus",
	}
}

// NewGraphiteType returns graphite datasource type
func NewGraphiteType() Type {
	return Type{
		name: "graphite",
	}
}

// NewRawType returns datasource type from raw string
// without validation.
func NewRawType(d string) Type {
	return Type{name: d}
}

// Get returns datasource type
func (t *Type) Get() string {
	return t.name
}

// Set changes datasource type
func (t *Type) Set(d Type) {
	t.name = d.name
}

// String implements String interface with default value.
func (t Type) String() string {
	if t.name == "" {
		return "prometheus"
	}
	return t.name
}

// ValidateExpr validates query expression with datasource ql.
func (t *Type) ValidateExpr(expr string) error {
	switch t.String() {
	case "graphite":
		if _, err := graphiteql.Parse(expr); err != nil {
			return fmt.Errorf("bad graphite expr: %q, err: %w", expr, err)
		}
	case "prometheus":
		if _, err := metricsql.Parse(expr); err != nil {
			return fmt.Errorf("bad prometheus expr: %q, err: %w", expr, err)
		}
	default:
		return fmt.Errorf("unknown datasource type=%q", t.name)
	}
	return nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (t *Type) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	if s == "" {
		s = "prometheus"
	}
	switch s {
	case "graphite", "prometheus":
	default:
		return fmt.Errorf("unknown datasource type=%q, want %q or %q", s, "prometheus", "graphite")
	}
	t.name = s
	return nil
}

// MarshalYAML implements the yaml.Unmarshaler interface.
func (t Type) MarshalYAML() (interface{}, error) {
	return t.name, nil
}
