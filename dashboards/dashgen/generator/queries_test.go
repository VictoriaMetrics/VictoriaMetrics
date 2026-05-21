package generator

import (
	"regexp"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/dashboards/dashgen/parser"
)

// TestSvcNameRegex ensures the current pattern extracts service name prefix
// from the vm_app_version label formats we see in real tags (all include date).
func TestSvcNameRegex(t *testing.T) {
	// svcNameRegex is double-escaped for PromQL; unescape for Go regexp.
	re := regexp.MustCompile(strings.ReplaceAll(svcNameRegex, `\\`, `\`))

	cases := []struct {
		version string
		expect  string
	}{
		{"operator-operator-20251031-152943-v0.65.0", "operator-operator"},
		{"victoria-logs-20251128-234103-tags-v1.39.0-0-ge4f2a3c0a0", "victoria-logs"},
		{"victoria-metrics-20251201-111831-tags-v1.131.0-enterprise-0-ge509c64054", "victoria-metrics"},
		{"vlagent-20251128-234216-tags-v1.39.0-0-ge4f2a3c0a0", "vlagent"},
		{"vmagent-20251201-112045-tags-v1.131.0-enterprise-0-ge509c64054", "vmagent"},
		{"vmalert-20251201-112310-tags-v1.131.0-enterprise-0-ge509c64054", "vmalert"},
		{"vmauth-20251017-122113-tags-v1.128.0-0-gf91789eebd", "vmauth"},
		{"vmbackupmanager-20251201-113731-tags-v1.131.0-enterprise-0-ge509c64054", "vmbackupmanager"},
		{"vminsert-20251201-114237-tags-v1.131.0-enterprise-cluster-0-g50309fe153", "vminsert"},
		{"vmselect-20251201-114427-tags-v1.131.0-enterprise-cluster-0-g50309fe153", "vmselect"},
		{"vmstorage-20251201-114630-tags-v1.131.0-enterprise-cluster-0-g50309fe153", "vmstorage"},
		{"vmanomaly-20251204-120000-tags-v1.0.0", "vmanomaly"},
	}

	for _, tc := range cases {
		m := re.FindStringSubmatch(tc.version)
		if len(m) < 2 {
			t.Fatalf("no match for version %q", tc.version)
		}
		got := m[1]
		if got != tc.expect {
			t.Errorf("svc name mismatch for %q: got %q, want %q", tc.version, got, tc.expect)
		}
	}
}

// TestComponentVersionPatterns verifies that version patterns correctly match expected components.
func TestComponentVersionPatterns(t *testing.T) {
	cases := []struct {
		component string
		versions  []string // versions that should match
	}{
		{"cluster", []string{"vminsert-20251201-114237", "vmselect-20251201-114427", "vmstorage-20251201-114630"}},
		{"single", []string{"victoria-metrics-20251201-111831"}},
		{"vmagent", []string{"vmagent-20251201-112045"}},
		{"vmalert", []string{"vmalert-20251201-112310"}},
		{"vmauth", []string{"vmauth-20251017-122113"}},
		{"vmanomaly", []string{"vmanomaly-20251204-120000"}},
		{"unknown", []string{"grafana-20251201", "telegraf-20251201"}},
	}

	for _, tc := range cases {
		pattern, ok := componentVersionPatterns[tc.component]
		if !ok {
			t.Errorf("component %q not found in componentVersionPatterns", tc.component)
			continue
		}

		re := regexp.MustCompile("^" + pattern)
		for _, version := range tc.versions {
			if !re.MatchString(version) {
				t.Errorf("pattern for %q should match %q, but didn't", tc.component, version)
			}
		}
	}
}

// TestComponentVersionPatternsNoFalsePositives verifies patterns don't match wrong components.
func TestComponentVersionPatternsNoFalsePositives(t *testing.T) {
	cases := []struct {
		component string
		versions  []string // versions that should NOT match
	}{
		{"cluster", []string{"vmagent-20251201", "vmalert-20251201", "victoria-metrics-20251201"}},
		{"single", []string{"vmagent-20251201", "vminsert-20251201"}},
		{"vmagent", []string{"vmalert-20251201", "vmauth-20251201"}},
		{"vmalert", []string{"vmagent-20251201", "vmauth-20251201"}},
		{"vmauth", []string{"vmagent-20251201", "vmalert-20251201"}},
		{"vmanomaly", []string{"vmagent-20251201", "vmalert-20251201"}},
	}

	for _, tc := range cases {
		pattern := componentVersionPatterns[tc.component]
		re := regexp.MustCompile("^" + pattern)
		for _, version := range tc.versions {
			if re.MatchString(version) {
				t.Errorf("pattern for %q should NOT match %q, but did", tc.component, version)
			}
		}
	}
}

// TestGenerateRefID verifies refId generation for various alert names.
func TestGenerateRefID(t *testing.T) {
	cases := []struct {
		alertName string
		want      string
	}{
		{"TooManyLogs", "TooManyLogs"},
		{"Too-Many-Logs", "TooManyLogs"},
		{"Too_Many_Logs", "TooManyLogs"},
		{"Too Many Logs", "TooManyLogs"},
		{"Alert:With:Colons", "AlertWithColons"},
		{"Alert.With.Dots", "AlertWithDots"},
		{"123StartWithDigit", "Q123StartWithDigit"},
		{"NormalAlert", "NormalAlert"},
		{"ConcurrentInsertsHitTheLimit", "ConcurrentInsertsHitTheLimit"},
	}

	for _, tc := range cases {
		got := GenerateRefID(tc.alertName)
		if got != tc.want {
			t.Errorf("GenerateRefID(%q) = %q, want %q", tc.alertName, got, tc.want)
		}
	}
}

// TestNormalizeAlertQuery verifies the query generation for different components.
func TestNormalizeAlertQuery(t *testing.T) {
	cases := []struct {
		rule           parser.AlertRule
		wantContains   []string
		wantNotContain []string
	}{
		{
			rule: parser.AlertRule{
				Alert:     "TestAlert",
				Expr:      "sum(rate(metric[5m])) > 0",
				Component: "vmagent",
			},
			wantContains: []string{
				"vmagent-.*",                // version filter
				"sum(rate(metric[5m])) > 0", // original expr preserved
				"min_over_time(",            // shows worst state over range
				"clamp_min(",                // prevents negative values
				"vm_app_version",            // joins with vm_app_version
				"svc_name",                  // extracts service name
				"$__range",                  // uses Grafana range variable
			},
		},
		{
			rule: parser.AlertRule{
				Alert:     "ClusterAlert",
				Expr:      "disk_usage > 0.9",
				Component: "cluster",
			},
			wantContains: []string{
				"(vminsert|vmselect|vmstorage)-.*", // cluster pattern
			},
		},
		{
			rule: parser.AlertRule{
				Alert:     "SingleAlert",
				Expr:      "memory_usage > 0.8",
				Component: "single",
			},
			wantContains: []string{
				"victoria-metrics-.*", // single pattern
			},
		},
		{
			rule: parser.AlertRule{
				Alert:     "UnknownAlert",
				Expr:      "some_metric > 0",
				Component: "unknown",
			},
			wantContains: []string{
				`version=~".*"`, // unknown matches all
			},
		},
	}

	for _, tc := range cases {
		got := NormalizeAlertQuery(tc.rule)

		for _, want := range tc.wantContains {
			if !strings.Contains(got, want) {
				t.Errorf("NormalizeAlertQuery(%q) should contain %q, got:\n%s", tc.rule.Alert, want, got)
			}
		}

		for _, notWant := range tc.wantNotContain {
			if strings.Contains(got, notWant) {
				t.Errorf("NormalizeAlertQuery(%q) should NOT contain %q", tc.rule.Alert, notWant)
			}
		}
	}
}

// TestNormalizeAlertQueryStructure verifies the overall structure of generated queries.
func TestNormalizeAlertQueryStructure(t *testing.T) {
	rule := parser.AlertRule{
		Alert:     "TestAlert",
		Expr:      "metric > 0",
		Component: "vmagent",
	}

	query := NormalizeAlertQuery(rule)

	// Verify min_over_time wrapper
	if !strings.HasPrefix(query, "min_over_time(") {
		t.Error("query should start with 'min_over_time('")
	}

	// Verify key components are present
	expectedParts := []string{
		"min_over_time(",
		"vm_svc = label_replace(",
		"total = count by (svc_name)",
		"firing_pod = count by (svc_name)",
		"firing_inst = count by (svc_name)",
		"firing = (firing_pod or firing_inst",
		"clamp_min(100 * (total - firing) / total, 0)",
		"[$__range:]",
	}

	for _, part := range expectedParts {
		if !strings.Contains(query, part) {
			t.Errorf("query missing expected part: %q", part)
		}
	}
}

// TestNormalizeAlertQueryExprPreserved verifies the original expression is preserved.
func TestNormalizeAlertQueryExprPreserved(t *testing.T) {
	expressions := []string{
		"sum(rate(http_requests_total[5m])) > 100",
		"avg(node_cpu_seconds_total) by (instance) > 0.9",
		`count(vm_app_version{version=~"vmagent.*"}) == 0`,
		"changes(process_start_time_seconds[1h]) > 2",
		"(disk_used / disk_total) > 0.95",
	}

	for _, expr := range expressions {
		rule := parser.AlertRule{
			Alert:     "TestAlert",
			Expr:      expr,
			Component: "vmagent",
		}

		query := NormalizeAlertQuery(rule)

		// The expression should appear twice (for pod join and instance join)
		count := strings.Count(query, expr)
		if count != 2 {
			t.Errorf("expression %q should appear exactly 2 times in query, found %d times", expr, count)
		}
	}
}

// TestNormalizeAlertQueryWhitespace verifies whitespace in expressions is handled.
func TestNormalizeAlertQueryWhitespace(t *testing.T) {
	rule := parser.AlertRule{
		Alert:     "TestAlert",
		Expr:      "  sum(rate(metric[5m])) > 0  ", // leading/trailing whitespace
		Component: "vmagent",
	}

	query := NormalizeAlertQuery(rule)

	// Should not contain the leading/trailing whitespace
	if strings.Contains(query, "  sum") {
		t.Error("query should trim leading whitespace from expression")
	}
}

// TestAllComponentsHavePatterns verifies all known components have version patterns.
func TestAllComponentsHavePatterns(t *testing.T) {
	requiredComponents := []string{
		"cluster",
		"single",
		"vmagent",
		"vmalert",
		"vmauth",
		"vmanomaly",
		"unknown",
	}

	for _, component := range requiredComponents {
		if _, ok := componentVersionPatterns[component]; !ok {
			t.Errorf("componentVersionPatterns missing entry for %q", component)
		}
	}
}
