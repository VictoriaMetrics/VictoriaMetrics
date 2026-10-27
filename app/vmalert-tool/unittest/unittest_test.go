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
	f := func(disableGroupLabel bool, files []string, externalLabels []string, externalURL, httpPort string, respectGroupInterval bool) {
		t.Helper()

		failed := UnitTest(files, disableGroupLabel, externalLabels, externalURL, httpPort, "", respectGroupInterval)
		if failed {
			t.Fatalf("unexpected failed test")
		}
	}

	// run multi files with random http port
	f(false, []string{"./testdata/test1.yaml", "./testdata/test2.yaml"}, []string{"cluster=prod"}, "http://grafana:3000", "", false)

	// every rule group is evaluated on its own interval
	f(false, []string{"./testdata/group-interval-test.yaml"}, nil, "", "", true)

	// every rule group is evaluated on the evaluation_interval
	f(false, []string{"./testdata/group-interval-default-test.yaml"}, nil, "", "", false)

	// disable group label
	// template with null external values
	// specify httpListenAddr
	f(true, []string{"./testdata/disable-group-label.yaml"}, nil, "", "8880", false)
}
