package storage

import (
	"bytes"
	"testing"

	"sigs.k8s.io/randfill"
)

func FuzzMetricName(f *testing.F) {
	f.Add([]byte("metric"))
	f.Add([]byte{0, 1, 2, 3})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		var mn MetricName
		randfill.NewFromGoFuzz(data).NumElements(0, 64).Fill(&mn)
		encoded := mn.Marshal(nil)
		var decoded MetricName
		if err := decoded.Unmarshal(encoded); err != nil {
			t.Fatalf("cannot unmarshal marshaled metric name: %v", err)
		}
		if got := decoded.Marshal(nil); !bytes.Equal(encoded, got) {
			t.Fatalf("metric name round trip changed encoding: %x -> %x", encoded, got)
		}
	})
}
