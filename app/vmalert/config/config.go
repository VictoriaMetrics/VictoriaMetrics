package config

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/metricsql"
)

// Group contains list of Rules grouped into
// entity with one name and evaluation interval
type Group struct {
	File        string
	Name        string        `yaml:"name"`
	Interval    time.Duration `yaml:"interval,omitempty"`
	Rules       []Rule        `yaml:"rules"`
	Concurrency int           `yaml:"concurrency"`

	// Catches all undefined fields and must be empty after parsing.
	XXX map[string]interface{} `yaml:",inline"`
}

// Validate check for internal Group or Rule configuration errors
func (g *Group) Validate(validateAnnotations bool) error {
	if g.Name == "" {
		return fmt.Errorf("group name must be set")
	}
	if len(g.Rules) == 0 {
		return fmt.Errorf("group %q can't contain no rules", g.Name)
	}
	uniqueRules := map[string]struct{}{}
	for _, r := range g.Rules {
		ruleName := r.Record
		if r.Alert != "" {
			ruleName = r.Alert
		}
		if _, ok := uniqueRules[ruleName]; ok {
			return fmt.Errorf("rule name %q duplicate", ruleName)
		}
		uniqueRules[ruleName] = struct{}{}
		if err := r.Validate(); err != nil {
			return fmt.Errorf("invalid rule %q.%q: %s", g.Name, ruleName, err)
		}
		if !validateAnnotations {
			continue
		}
		if err := notifier.ValidateTemplates(r.Annotations); err != nil {
			return fmt.Errorf("invalid annotations for rule %q.%q: %s", g.Name, ruleName, err)
		}
		if err := notifier.ValidateTemplates(r.Labels); err != nil {
			return fmt.Errorf("invalid labels for rule %q.%q: %s", g.Name, ruleName, err)
		}
	}
	return checkOverflow(g.XXX, fmt.Sprintf("group %q", g.Name))
}

// Rule describes entity that represent either
// recording rule or alerting rule.
type Rule struct {
	Record      string            `yaml:"record,omitempty"`
	Alert       string            `yaml:"alert,omitempty"`
	Expr        string            `yaml:"expr"`
	For         time.Duration     `yaml:"for,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty"`
}

// Validate check for Rule configuration errors
func (r *Rule) Validate() error {
	if (r.Record == "" && r.Alert == "") || (r.Record != "" && r.Alert != "") {
		return fmt.Errorf("either `record` or `alert` must be set")
	}
	if r.Expr == "" {
		return fmt.Errorf("expression can't be empty")
	}
	if _, err := metricsql.Parse(r.Expr); err != nil {
		return fmt.Errorf("invalid expression: %w", err)
	}
	return nil
}

// Parse parses rule configs from given file patterns
func Parse(pathPatterns []string, validateAnnotations bool) ([]Group, error) {
	var fp []string
	for _, pattern := range pathPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("error reading file pattern %s: %v", pattern, err)
		}
		fp = append(fp, matches...)
	}
	var groups []Group
	for _, file := range fp {
		uniqueGroups := map[string]struct{}{}
		gr, err := parseFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to parse file %q: %w", file, err)
		}
		for _, g := range gr {
			if err := g.Validate(validateAnnotations); err != nil {
				return nil, fmt.Errorf("invalid group %q in file %q: %s", g.Name, file, err)
			}
			if _, ok := uniqueGroups[g.Name]; ok {
				return nil, fmt.Errorf("group name %q duplicate in file %q", g.Name, file)
			}
			uniqueGroups[g.Name] = struct{}{}
			g.File = file
			groups = append(groups, g)
		}
	}
	if len(groups) < 1 {
		return nil, fmt.Errorf("no groups found in %s", strings.Join(pathPatterns, ";"))
	}
	return groups, nil
}

func parseFile(path string) ([]Group, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading alert rule file: %w", err)
	}
	g := struct {
		Groups []Group `yaml:"groups"`
		// Catches all undefined fields and must be empty after parsing.
		XXX map[string]interface{} `yaml:",inline"`
	}{}
	err = yaml.Unmarshal(data, &g)
	if err != nil {
		return nil, err
	}
	return g.Groups, checkOverflow(g.XXX, "config")
}

func checkOverflow(m map[string]interface{}, ctx string) error {
	if len(m) > 0 {
		var keys []string
		for k := range m {
			keys = append(keys, k)
		}
		return fmt.Errorf("unknown fields in %s: %s", ctx, strings.Join(keys, ", "))
	}
	return nil
}
