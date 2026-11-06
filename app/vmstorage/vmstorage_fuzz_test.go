package vmstorage

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
	"sigs.k8s.io/randfill"
)

func FuzzMetricBlock(f *testing.F) {
	f.Add([]byte("metric"))
	f.Add([]byte{0, 1, 2, 3})
	f.Add(marshalDefault(nil, &storage.MetricBlock{MetricName: []byte("metric")}))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}
		checkMetricBlock(t, data, false)
	})
}

func FuzzStructuredMetricBlock(f *testing.F) {
	f.Add([]byte("metric"))
	f.Add([]byte{0, 1, 2, 3})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		var (
			generated             storage.MetricBlock
			tsid                  storage.TSID
			timestamp, value      int64
			scale                 int16
			timestampsBlockOffset uint64
			valuesBlockOffset     uint64
			precisionBits         uint8
		)

		// Build a valid one-row block from random data.
		r := randfill.NewFromGoFuzz(data).NumElements(0, 64)

		r.Fill(&generated.MetricName)
		r.Fill(&tsid)
		r.Fill(&timestamp)
		r.Fill(&value)
		r.Fill(&scale)
		r.Fill(&timestampsBlockOffset)
		r.Fill(&valuesBlockOffset)
		r.Fill(&precisionBits)

		// Valid precisionBits range is [1, 64]
		precisionBits = precisionBits%64 + 1

		generated.Block.Init(&tsid, []int64{timestamp}, []int64{value}, scale, precisionBits)
		generated.Block.MarshalData(timestampsBlockOffset, valuesBlockOffset)
		checkMetricBlock(t, marshalDefault(nil, &generated), true)
	})
}

func checkMetricBlock(t *testing.T, data []byte, strict bool) {
	t.Helper()
	var mb storage.MetricBlock
	tail, err := mb.Unmarshal(data)
	if err != nil || len(tail) != 0 {
		if strict {
			t.Fatalf("cannot unmarshal generated metric block: %v; tail: %x", err, tail)
		}
		return
	}
	// Compare canonical encodings; raw input may use non-canonical lengths.
	canonical := marshalDefault(nil, &mb)
	var decoded storage.MetricBlock
	tail, err = decoded.Unmarshal(canonical)
	if err != nil || len(tail) != 0 {
		t.Fatalf("cannot unmarshal canonical metric block: %v; tail: %x", err, tail)
	}
	if got := marshalDefault(nil, &decoded); !bytes.Equal(got, canonical) {
		t.Fatalf("metric block canonical encoding changed: %x -> %x", canonical, got)
	}
}
