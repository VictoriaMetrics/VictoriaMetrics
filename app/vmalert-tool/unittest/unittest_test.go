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

		failed := UnitTest(files, false)
		if !failed {
			t.Fatalf("expecting failed test")
		}
	}

	// failing test
	f([]string{"./testdata/failed-test.yaml"})
}

func TestUnitTest_Success(t *testing.T) {
	f := func(disableGroupLabel bool, files []string) {
		t.Helper()

		failed := UnitTest(files, disableGroupLabel)
		if failed {
			t.Fatalf("unexpected failed test")
		}
	}

	// run multi files
	f(false, []string{"./testdata/test1.yaml", "./testdata/test2.yaml"})

	// disable group label
	f(true, []string{"./testdata/disable-group-label.yaml"})
}
