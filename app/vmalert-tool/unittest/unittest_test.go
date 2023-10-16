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

func TestUnitRule(t *testing.T) {
	testCases := []struct {
		name              string
		disableGroupLabel bool
		files             []string
		failed            bool
	}{
		{
			name:   "run multi files",
			files:  []string{"./testdata/test1.yaml", "./testdata/test2.yaml"},
			failed: false,
		},
		{
			name:              "disable group label",
			disableGroupLabel: true,
			files:             []string{"./testdata/disable-group-label.yaml"},
			failed:            false,
		},
		{
			name:   "failing test",
			files:  []string{"./testdata/failed-test.yaml"},
			failed: true,
		},
	}
	for _, tc := range testCases {
		fail := UnitTest(tc.files, tc.disableGroupLabel)
		if fail != tc.failed {
			t.Fatalf("failed to test %s, expect %t, got %t", tc.name, tc.failed, fail)
		}
	}
}
