package common

import (
	"errors"
	"fmt"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/metricsql"
)

// Rule is basic alert entity
type Rule struct {
	Name        string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         time.Duration     `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

// Validate validates rule
func (r Rule) Validate() error {
	if r.Name == "" {
		return errors.New("rule name can not be empty")
	}
	if r.Expr == "" {
		return fmt.Errorf("rule %s expression can not be empty", r.Name)
	}
	if _, err := metricsql.Parse(r.Expr); err != nil {
		return fmt.Errorf("rule %s invalid expression: %w", r.Name, err)
	}
	return nil
}

// Group grouping array of alert
type Group struct {
	Name  string
	Rules []Rule
}
