package generator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/dashboards/dashgen/parser"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestQuickTemplateMatchesJsonnet(t *testing.T) {
	alertsDir := filepath.Join("..", "..", "..", "deployment", "docker", "rules")
	rules, err := parser.ParseAlertDirectory(alertsDir)
	if err != nil {
		t.Fatalf("parse alerts: %v", err)
	}

	if len(rules) == 0 {
		t.Fatal("no alert rules parsed")
	}

	sort.Slice(rules, func(i, j int) bool { return rules[i].Alert < rules[j].Alert })

	alertDefs := make([]AlertDefinition, 0, len(rules))
	renames := make(map[string]string, len(rules))
	for _, r := range rules {
		prefix := r.Component
		if prefix == "unknown" {
			prefix = "ALL"
		}
		refID := GenerateRefID(prefix + "_" + r.Alert)
		expr := NormalizeAlertQuery(r)
		alertDefs = append(alertDefs, AlertDefinition{RefID: refID, Expr: expr})

		fieldName := "Value #" + refID
		displayName := prefix + ": " + r.Alert
		renames[fieldName] = displayName
	}

	qtplJSON, err := RenderWithQuickTemplate(alertDefs, renames, "VictoriaMetrics - Status Page", "vm-status-page")
	if err != nil {
		t.Fatalf("quicktemplate render: %v", err)
	}

	// Baseline: existing generated dashboard in repo (publishable artifact).
	baselinePath := filepath.Join("..", "..", "status-page-generated.json")
	baselineBytes, err := os.ReadFile(baselinePath)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("baseline %s not present; generate via dashgen and commit", baselinePath)
		}
		t.Fatalf("read baseline: %v", err)
	}

	var qtObj, baselineObj interface{}
	if err := json.Unmarshal([]byte(qtplJSON), &qtObj); err != nil {
		t.Fatalf("unmarshal quicktemplate output: %v", err)
	}
	if err := json.Unmarshal(baselineBytes, &baselineObj); err != nil {
		t.Fatalf("unmarshal baseline output: %v", err)
	}

	if diff := cmp.Diff(baselineObj, qtObj, cmpopts.EquateApprox(0, 1e-9)); diff != "" {
		t.Fatalf("quicktemplate output differs from baseline (-want +got):\n%s", diff)
	}
}
