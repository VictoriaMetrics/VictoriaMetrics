package tests

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"testing"
)

func TestSingleMaxIngestionRate(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	sut := tc.MustStartVmsingle("vmsingle", []string{"-maxIngestionRate=10000"})
	print(sut)
}
