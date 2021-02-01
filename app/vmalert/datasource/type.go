package datasource

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/graphiteql"
	"github.com/VictoriaMetrics/metricsql"
)

const graphiteType = "graphite"
const prometheusType = "prometheus"

// Type represents data source type
type Type struct {
	name string
}

// NewPrometheusType returns prometheus datasource type
func NewPrometheusType() Type {
	return Type{name: prometheusType}
}

// NewGraphiteType returns graphite datasource type
func NewGraphiteType() Type {
	return Type{name: graphiteType}
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
		return prometheusType
	}
	return t.name
}

// ValidateExpr validates query expression with datasource ql.
func (t *Type) ValidateExpr(expr string) error {
	switch t.name {
	case graphiteType:
		if _, err := graphiteql.Parse(expr); err != nil {
			return fmt.Errorf("bad graphite expr: %q, err: %w", expr, err)
		}
	case "", prometheusType:
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
	switch s {
	case "":
		s = prometheusType
	case graphiteType, prometheusType:
	default:
		return fmt.Errorf("unknown datasource type=%q, want %q or %q", s, prometheusType, graphiteType)
	}
	t.name = s
	return nil
}

// MarshalYAML implements the yaml.Unmarshaler interface.
func (t Type) MarshalYAML() (interface{}, error) {
	return t.name, nil
}
