package unittest

import (
	"os"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
)

func TestMain(m *testing.M) {
	if err := templates.Load([]string{}, true); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestUnitTest_Failure(t *testing.T) {
	f := func(files []string) {
		t.Helper()

		failed := UnitTest(files, false, nil, "")
		if !failed {
			t.Fatalf("expecting failed test")
		}
	}

	// failing test
	f([]string{"./testdata/failed-test.yaml"})
}

func TestUnitTest_Success(t *testing.T) {
	f := func(disableGroupLabel bool, files []string, externalLabels []string, externalURL string) {
		t.Helper()

		failed := UnitTest(files, disableGroupLabel, externalLabels, externalURL)
		if failed {
			t.Fatalf("unexpected failed test")
		}
	}

	// run multi files
	f(false, []string{"./testdata/test1.yaml", "./testdata/test2.yaml"}, []string{"cluster=prod"}, "http://grafana:3000")

	// disable group label
	// template with null external values
	f(true, []string{"./testdata/disable-group-label.yaml"}, nil, "")
}
