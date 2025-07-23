package metricsmetadata

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkMetadataMarshal(b *testing.B) {
	b.ReportAllocs()

	m := &prompb.MetricMetadata{
		Type:             3,
		MetricFamilyName: "test_family",
		Help:             "test_help",
		Unit:             "test_unit",
	}

	dst := make([]byte, 0, 256)
	for i := 0; i < b.N; i++ {
		data := MarshalRow(dst, 0, 0, m)
		if len(data) == 0 {
			b.Fatalf("unexpected empty data after marshaling")
		}
	}
}

// BenchmarkMarshalUnmarshal benchmarks the Marshal and Unmarshal functions for MetricMetadata.
func BenchmarkMetadataMarshalUnmarshal(b *testing.B) {
	b.ReportAllocs()

	m := &prompb.MetricMetadata{
		Type:             3,
		MetricFamilyName: "test_family",
		Help:             "test_help",
		Unit:             "test_unit",
	}

	data := MarshalRow(nil, 0, 0, m)

	b.ResetTimer()

	var mr Row
	for i := 0; i < b.N; i++ {
		if _, err := mr.Unmarshal(data); err != nil {
			b.Fatalf("unexpected error during unmarshaling: %s", err)
		}
	}
}
