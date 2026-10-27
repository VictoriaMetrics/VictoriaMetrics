package unittest

import (
	"testing"
)

func TestUnitTest_Failure(t *testing.T) {
	f := func(files []string) {
		t.Helper()

		failed := UnitTest(files, false, nil, "", "", "", false)
		if !failed {
			t.Fatalf("expecting failed test")
		}
	}

	f([]string{"./testdata/failed-test-with-missing-rulefile.yaml"})

	f([]string{"./testdata/failed-test.yaml"})
}

func TestUnitTest_Success(t *testing.T) {
	f := func(disableGroupLabel bool, files []string, externalLabels []string, externalURL, httpPort string) {
		t.Helper()

		failed := UnitTest(files, disableGroupLabel, externalLabels, externalURL, httpPort, "", false)
		if failed {
			t.Fatalf("unexpected failed test")
		}
	}

	// run multi files with random http port
	f(false, []string{"./testdata/test1.yaml", "./testdata/test2.yaml"}, []string{"cluster=prod"}, "http://grafana:3000", "")

	// disable group label
	// template with null external values
	// specify httpListenAddr
	f(true, []string{"./testdata/disable-group-label.yaml"}, nil, "", "8880")
}

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

	// Rule groups depending on each other through recording rule chains, the ALERTS
	// series, a selector without a metric name, a template query and a shared alert name.
	f([]string{"./testdata/rule-deps-test.yaml"}, nil, "", false)

	f([]string{"./testdata/test1.yaml"}, []string{"cluster=prod"}, "http://grafana:3000", false)

	f([]string{"./testdata/test2.yaml"}, []string{"cluster=prod"}, "http://grafana:3000", false)

	// A failing test must keep failing when the unrelated groups are skipped.
	f([]string{"./testdata/failed-test.yaml"}, nil, "", true)
}
