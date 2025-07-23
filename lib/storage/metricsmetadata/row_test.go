package metricsmetadata

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestMarshalUnmarshal(t *testing.T) {
	m := &prompb.MetricMetadata{
		Type:             3,
		MetricFamilyName: "test_family",
		Help:             "test_help",
		Unit:             "test_unit",
	}

	data := MarshalRow(nil, 0, 0, m)
	if len(data) == 0 {
		t.Fatalf("unexpected empty data after marshaling")
	}

	var mr Row
	if _, err := mr.Unmarshal(data); err != nil {
		t.Fatalf("unexpected error during unmarshaling: %s", err)
	}

	if mr.AccountID != 0 || mr.ProjectID != 0 || mr.Type != m.Type ||
		string(mr.MetricFamilyName) != m.MetricFamilyName ||
		string(mr.Help) != m.Help || string(mr.Unit) != m.Unit {
		t.Fatalf("unmarshaled data does not match original: got %+v, want %+v", mr, m)
	}

}
