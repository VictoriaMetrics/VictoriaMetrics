package unittest

import (
	"testing"
)

// TestUnitTest_SkipUnrelatedRuleGroups checks that narrowing the evaluated rule
// groups doesn't change the outcome of a test file.
func TestUnitTest_SkipUnrelatedRuleGroups(t *testing.T) {
	f := func(files []string, externalLabels []string, externalURL string, failedExpected bool) {
		t.Helper()

		failedFull := UnitTest(files, false, externalLabels, externalURL, "", "", false)
		failedSkipped := UnitTest(files, false, externalLabels, externalURL, "", "", true)

		if failedFull != failedExpected {
			t.Fatalf("unexpected result with all rule groups for %q; got failed=%v; want %v", files, failedFull, failedExpected)
		}
		if failedSkipped != failedFull {
			t.Fatalf("skipping unrelated rule groups changed the result of %q; got failed=%v; want %v", files, failedSkipped, failedFull)
		}
	}

	// Rule groups depending on each other through recording rule chains, the
	// ALERTS series, an unanalysable selector and a shared alert name.
	f([]string{"./testdata/rule-deps-test.yaml"}, nil, "", false)

	f([]string{"./testdata/test1.yaml"}, []string{"cluster=prod"}, "http://grafana:3000", false)

	f([]string{"./testdata/test2.yaml"}, []string{"cluster=prod"}, "http://grafana:3000", false)

	// A failing test must keep failing when the unrelated groups are skipped.
	f([]string{"./testdata/failed-test.yaml"}, nil, "", true)
}
