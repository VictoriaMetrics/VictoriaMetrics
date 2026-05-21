package parser

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectComponent(t *testing.T) {
	cases := []struct {
		groupName string
		want      string
	}{
		// Exact matches (lowercase)
		{"vmcluster", "cluster"},
		{"vmsingle", "single"},
		{"vmagent", "vmagent"},
		{"vmalert", "vmalert"},
		{"vmauth", "vmauth"},
		{"vmanomaly", "vmanomaly"},

		// Case insensitive
		{"VMCluster", "cluster"},
		{"VMSingle", "single"},
		{"VMAgent", "vmagent"},

		// Substring matches
		{"cluster-alerts", "cluster"},
		{"single-node-alerts", "single"},
		{"vmagent-recording", "vmagent"},
		{"vmalert-errors", "vmalert"},
		{"vmauth-health", "vmauth"},
		{"vmanomaly-detection", "vmanomaly"},
		{"anomaly-alerts", "vmanomaly"},

		// Unknown fallback - group names that don't match any component
		{"other-alerts", "unknown"},
	}

	for _, tc := range cases {
		got := detectComponent(tc.groupName)
		if got != tc.want {
			t.Errorf("detectComponent(%q) = %q, want %q", tc.groupName, got, tc.want)
		}
	}
}

func TestParseAlertFile(t *testing.T) {
	// Create a temporary test file
	content := `groups:
  - name: vmagent
    rules:
      - alert: TooManyLogs
        expr: sum(rate(vm_log_messages_total{level="error"}[5m])) > 0
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Too many error logs"
      - alert: TooManyRestarts
        expr: changes(process_start_time_seconds[1h]) > 2
        for: 5m
  - name: vmcluster
    interval: 30s
    rules:
      - alert: DiskRunsOutOfSpace
        expr: vm_free_disk_space_bytes / vm_data_size_bytes < 0.1
        labels:
          severity: critical
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-alerts.yml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	alertFile, err := ParseAlertFile(tmpFile)
	if err != nil {
		t.Fatalf("ParseAlertFile failed: %v", err)
	}

	// Verify groups count
	if len(alertFile.Groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(alertFile.Groups))
	}

	// Verify first group
	if alertFile.Groups[0].Name != "vmagent" {
		t.Errorf("expected group name 'vmagent', got %q", alertFile.Groups[0].Name)
	}
	if len(alertFile.Groups[0].Rules) != 2 {
		t.Errorf("expected 2 rules in vmagent group, got %d", len(alertFile.Groups[0].Rules))
	}

	// Verify component detection
	if alertFile.Groups[0].Rules[0].Component != "vmagent" {
		t.Errorf("expected component 'vmagent', got %q", alertFile.Groups[0].Rules[0].Component)
	}
	if alertFile.Groups[1].Rules[0].Component != "cluster" {
		t.Errorf("expected component 'cluster', got %q", alertFile.Groups[1].Rules[0].Component)
	}

	// Verify alert fields
	rule := alertFile.Groups[0].Rules[0]
	if rule.Alert != "TooManyLogs" {
		t.Errorf("expected alert 'TooManyLogs', got %q", rule.Alert)
	}
	if rule.For != "15m" {
		t.Errorf("expected for '15m', got %q", rule.For)
	}
	if rule.Labels["severity"] != "warning" {
		t.Errorf("expected severity 'warning', got %q", rule.Labels["severity"])
	}
}

func TestParseAlertFileNotFound(t *testing.T) {
	_, err := ParseAlertFile("/nonexistent/path/file.yml")
	if err == nil {
		t.Error("expected error for nonexistent file, got nil")
	}
}

func TestParseAlertFileInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.yml")
	if err := os.WriteFile(tmpFile, []byte("invalid: yaml: content: ["), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err := ParseAlertFile(tmpFile)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestParseAlertDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple alert files
	file1 := `groups:
  - name: vmagent
    rules:
      - alert: Alert1
        expr: metric1 > 0
`
	file2 := `groups:
  - name: vmalert
    rules:
      - alert: Alert2
        expr: metric2 > 0
      - alert: Alert3
        expr: metric3 > 0
`
	// Non-YAML file should be ignored
	nonYaml := "this is not yaml at all"

	if err := os.WriteFile(filepath.Join(tmpDir, "vmagent.yml"), []byte(file1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "vmalert.yaml"), []byte(file2), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte(nonYaml), 0644); err != nil {
		t.Fatal(err)
	}

	rules, err := ParseAlertDirectory(tmpDir)
	if err != nil {
		t.Fatalf("ParseAlertDirectory failed: %v", err)
	}

	if len(rules) != 3 {
		t.Errorf("expected 3 rules, got %d", len(rules))
	}

	// Verify components are set correctly
	componentCounts := make(map[string]int)
	for _, rule := range rules {
		componentCounts[rule.Component]++
	}

	if componentCounts["vmagent"] != 1 {
		t.Errorf("expected 1 vmagent rule, got %d", componentCounts["vmagent"])
	}
	if componentCounts["vmalert"] != 2 {
		t.Errorf("expected 2 vmalert rules, got %d", componentCounts["vmalert"])
	}
}

func TestParseAlertDirectoryNotFound(t *testing.T) {
	_, err := ParseAlertDirectory("/nonexistent/directory")
	if err == nil {
		t.Error("expected error for nonexistent directory, got nil")
	}
}

func TestAlertRuleFields(t *testing.T) {
	content := `groups:
  - name: test
    rules:
      - alert: CompleteAlert
        expr: metric > threshold
        for: 10m
        labels:
          severity: critical
          team: platform
        annotations:
          summary: "Alert summary"
          description: "Alert description"
`
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "complete.yml")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	alertFile, err := ParseAlertFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	rule := alertFile.Groups[0].Rules[0]

	// Verify all fields
	if rule.Alert != "CompleteAlert" {
		t.Errorf("Alert = %q, want 'CompleteAlert'", rule.Alert)
	}
	if rule.Expr != "metric > threshold" {
		t.Errorf("Expr = %q, want 'metric > threshold'", rule.Expr)
	}
	if rule.For != "10m" {
		t.Errorf("For = %q, want '10m'", rule.For)
	}
	if rule.Labels["severity"] != "critical" {
		t.Errorf("Labels[severity] = %q, want 'critical'", rule.Labels["severity"])
	}
	if rule.Labels["team"] != "platform" {
		t.Errorf("Labels[team] = %q, want 'platform'", rule.Labels["team"])
	}
	if rule.Annotations["summary"] != "Alert summary" {
		t.Errorf("Annotations[summary] = %q, want 'Alert summary'", rule.Annotations["summary"])
	}
	if rule.GroupName != "test" {
		t.Errorf("GroupName = %q, want 'test'", rule.GroupName)
	}
}
