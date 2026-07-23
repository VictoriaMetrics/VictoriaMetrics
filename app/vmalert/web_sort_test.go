package main

import (
	"cmp"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
)

func TestCompareIDs(t *testing.T) {
	t.Parallel()
	tests := []struct {
		a, b string
		want int
	}{
		{"1", "2", -1},
		{"2", "1", 1},
		{"1", "1", 0},
		{"2", "10", -1}, // numeric: 2 < 10, lexicographic would be wrong
		{"10", "2", 1},
		{"100", "99", 1},
	}
	for _, tc := range tests {
		got := compareIDs(tc.a, tc.b)
		if cmp.Compare(got, tc.want) != 0 {
			t.Fatalf("compareIDs(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSortRulesByEvaluationTime(t *testing.T) {
	t.Parallel()

	rules := []rule.ApiRule{
		{Name: "a", EvaluationTime: 0.1, ID: "1"},
		{Name: "a", EvaluationTime: 0.1, ID: "2"},
		{Name: "slow", EvaluationTime: 3.5, ID: "3"},
		{Name: "medium", EvaluationTime: 1.2, ID: "4"},
		{Name: "fast", EvaluationTime: 0.1, ID: "10"}, // ID "10" must sort after "2" numerically
	}
	sortRulesByEvaluationTime(rules)

	want := []string{"slow", "medium", "a", "a", "fast"}
	wantIDs := []string{"3", "4", "1", "2", "10"}
	for i, name := range want {
		if rules[i].Name != name {
			t.Fatalf("unexpected name at index %d: got %q, want %q", i, rules[i].Name, name)
		}
		if rules[i].ID != wantIDs[i] {
			t.Fatalf("unexpected ID at index %d: got %q, want %q", i, rules[i].ID, wantIDs[i])
		}
	}
}

func TestSortGroupsByEvaluationTime(t *testing.T) {
	t.Parallel()

	groups := []*rule.ApiGroup{
		{
			Name: "group-a",
			Rules: []rule.ApiRule{
				{EvaluationTime: 0.5},
			},
		},
		{
			Name: "group-b",
			Rules: []rule.ApiRule{
				{EvaluationTime: 2.0},
				{EvaluationTime: 1.0},
			},
		},
		{
			Name: "group-c",
			Rules: []rule.ApiRule{
				{EvaluationTime: 1.5},
			},
		},
	}
	sortGroupsByEvaluationTime(groups)

	want := []string{"group-b", "group-c", "group-a"}
	for i, name := range want {
		if groups[i].Name != name {
			t.Fatalf("unexpected order at index %d: got %q, want %q", i, groups[i].Name, name)
		}
	}
}


