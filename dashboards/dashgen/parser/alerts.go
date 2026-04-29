package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// AlertRule represents a single alert rule from a Prometheus/VictoriaMetrics alert YAML file.
type AlertRule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	For         string            `yaml:"for"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`

	// Derived fields (not from YAML)
	Component string // Component this alert belongs to (cluster, single, vmagent, etc.)
	GroupName string // Name of the alert group
}

// AlertGroup represents a group of alert rules.
type AlertGroup struct {
	Name        string      `yaml:"name"`
	Interval    string      `yaml:"interval"`
	Concurrency int         `yaml:"concurrency"`
	Rules       []AlertRule `yaml:"rules"`
}

// AlertFile represents the structure of an alert YAML file.
type AlertFile struct {
	Groups []AlertGroup `yaml:"groups"`
}

// ParseAlertFile parses a single alert YAML file and returns the parsed structure.
func ParseAlertFile(path string) (*AlertFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var alertFile AlertFile
	if err := yaml.Unmarshal(data, &alertFile); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	// Derive component name from group name for each rule
	for i := range alertFile.Groups {
		component := detectComponent(alertFile.Groups[i].Name)
		for j := range alertFile.Groups[i].Rules {
			alertFile.Groups[i].Rules[j].Component = component
			alertFile.Groups[i].Rules[j].GroupName = alertFile.Groups[i].Name
		}
	}

	return &alertFile, nil
}

// ParseAlertDirectory parses all .yml/.yaml files in a directory
// and returns a flat list of all alert rules.
func ParseAlertDirectory(dir string) ([]AlertRule, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	var allRules []AlertRule
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml") {
			continue
		}

		alertFile, err := ParseAlertFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}

		for _, group := range alertFile.Groups {
			allRules = append(allRules, group.Rules...)
		}
	}

	return allRules, nil
}

// componentMapping defines exact group name to component mappings.
var componentMapping = map[string]string{
	"vmcluster": "cluster",
	"vmsingle":  "single",
	"vmagent":   "vmagent",
	"vmalert":   "vmalert",
	"vmauth":    "vmauth",
	"vmanomaly": "vmanomaly",
}

// detectComponent determines the component type from the group name.
// Returns "unknown" if the component cannot be determined.
func detectComponent(groupName string) string {
	groupLower := strings.ToLower(groupName)

	// Check exact match first
	if component, ok := componentMapping[groupLower]; ok {
		return component
	}

	// Fallback to substring matching (order matters: more specific first)
	switch {
	case strings.Contains(groupLower, "cluster"):
		return "cluster"
	case strings.Contains(groupLower, "single"):
		return "single"
	case strings.Contains(groupLower, "vmanomaly"), strings.Contains(groupLower, "anomaly"):
		return "vmanomaly"
	case strings.Contains(groupLower, "vmalert"): // Must be before "alert" check
		return "vmalert"
	case strings.Contains(groupLower, "vmagent"), strings.Contains(groupLower, "agent"):
		return "vmagent"
	case strings.Contains(groupLower, "vmauth"), strings.Contains(groupLower, "auth"):
		return "vmauth"
	default:
		return "unknown"
	}
}
