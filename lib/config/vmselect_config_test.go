package config

import "testing"

func TestNilPointer(t *testing.T) {
	labelNames := VMSelectTreatDotsAsIsLabels.Load()
	if labelNames == nil || len(*labelNames) == 0 {
		t.Log("len == 0")
		return
	}
	for _, ln := range *labelNames {
		t.Log("ln: ", ln)
	}
}

func TestInvokeNilPointerMethod(t *testing.T) {
	labelNames := VMSelectTreatDotsAsIsLabels.Load()
	if labelNames == nil {
		t.Log("nil")
		return
	}
	if labelNames.Contains("hello") {
		t.Log("contains")
	}
}
