package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/VictoriaMetrics/VictoriaMetrics/dashboards/dashgen/generator"
	"github.com/VictoriaMetrics/VictoriaMetrics/dashboards/dashgen/parser"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	alertsDir := flag.String("alerts-dir", "", "Path to directory with alert YAML files")
	outputFile := flag.String("output", "dashboard.json", "Path to output JSON file")
	title := flag.String("title", "VictoriaMetrics - Status Page", "Dashboard title")
	uid := flag.String("uid", "vm-status-page", "Dashboard UID")

	flag.Parse()

	if *alertsDir == "" {
		return fmt.Errorf("--alerts-dir is required")
	}

	fmt.Printf("Parsing alert files from: %s\n", *alertsDir)

	allRules, err := parser.ParseAlertDirectory(*alertsDir)
	if err != nil {
		return fmt.Errorf("parse alerts: %w", err)
	}

	fmt.Printf("Found %d alert rules\n", len(allRules))

	if len(allRules) == 0 {
		return fmt.Errorf("no alert rules found in %s", *alertsDir)
	}

	// Sort rules for deterministic output
	sort.Slice(allRules, func(i, j int) bool {
		return allRules[i].Alert < allRules[j].Alert
	})

	// Prepare data for rendering
	alerts, renames, err := buildAlertData(allRules)
	if err != nil {
		return fmt.Errorf("build alert data: %w", err)
	}

	jsonOutput, err := generator.RenderWithQuickTemplate(alerts, renames, *title, *uid)
	if err != nil {
		return fmt.Errorf("render quicktemplate: %w", err)
	}

	// Write output
	if err := os.WriteFile(*outputFile, []byte(jsonOutput), 0644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}

	fmt.Printf("\n✓ Dashboard generated successfully: %s\n", *outputFile)
	return nil
}

// buildAlertData converts parsed alert rules into Jsonnet-compatible data structures.
// Returns error if any alert has empty name or expression.
func buildAlertData(rules []parser.AlertRule) ([]generator.AlertDefinition, map[string]string, error) {
	alerts := make([]generator.AlertDefinition, 0, len(rules))
	renames := make(map[string]string, len(rules))

	for _, rule := range rules {
		// Validate required fields
		if rule.Alert == "" {
			return nil, nil, fmt.Errorf("alert in group %q has empty name", rule.GroupName)
		}
		if rule.Expr == "" {
			return nil, nil, fmt.Errorf("alert %q has empty expression", rule.Alert)
		}

		prefix := rule.Component
		if prefix == "unknown" {
			prefix = "ALL"
		}

		refID := generator.GenerateRefID(prefix + "_" + rule.Alert)
		query := generator.NormalizeAlertQuery(rule)

		alerts = append(alerts, generator.AlertDefinition{
			RefID: refID,
			Expr:  query,
		})

		// Grafana uses "Value #<refID>" as the field name
		fieldName := fmt.Sprintf("Value #%s", refID)
		displayName := fmt.Sprintf("%s: %s", prefix, rule.Alert)
		renames[fieldName] = displayName
	}

	return alerts, renames, nil
}
