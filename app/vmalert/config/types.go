package config

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/graphiteql"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/metricsql"
)

// Type represents data source type
type Type struct {
	Name string
}

// NewPrometheusType returns prometheus datasource type
func NewPrometheusType() Type {
	return Type{
		Name: "prometheus",
	}
}

// NewGraphiteType returns graphite datasource type
func NewGraphiteType() Type {
	return Type{
		Name: "graphite",
	}
}

// NewVLogsType returns victorialogs datasource type
func NewVLogsType() Type {
	return Type{
		Name: "vlogs",
	}
}

// NewRawType returns datasource type from raw string
// without validation.
func NewRawType(d string) Type {
	return Type{Name: d}
}

// Get returns datasource type
func (t *Type) Get() string {
	return t.Name
}

// Set changes datasource type
func (t *Type) Set(d Type) {
	t.Name = d.Name
}

// String implements String interface with default value.
func (t Type) String() string {
	if t.Name == "" {
		return "prometheus"
	}
	return t.Name
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
	case "vlogs":
		if _, err := logstorage.ParseStatsQuery(expr); err != nil {
			return fmt.Errorf("bad LogsQL expr: %q, err: %w", expr, err)
		}
	default:
		return fmt.Errorf("unknown datasource type=%q", t.Name)
	}
	return nil
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (t *Type) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	switch s {
	case "graphite", "prometheus", "vlogs":
	default:
		return fmt.Errorf("unknown datasource type=%q, want prometheus, graphite or vlogs", s)
	}
	t.Name = s
	return nil
}

// MarshalYAML implements the yaml.Unmarshaler interface.
func (t Type) MarshalYAML() (any, error) {
	return t.Name, nil
}

// Header is a Key - Value struct for holding an HTTP header.
type Header struct {
	Key   string
	Value string
}

// UnmarshalYAML implements the yaml.Unmarshaler interface.
func (h *Header) UnmarshalYAML(unmarshal func(any) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	if s == "" {
		return nil
	}
	n := strings.IndexByte(s, ':')
	if n < 0 {
		return fmt.Errorf(`missing ':' in header %q; expecting "key: value" format`, s)
	}
	h.Key = strings.TrimSpace(s[:n])
	h.Value = strings.TrimSpace(s[n+1:])
	return nil
}
