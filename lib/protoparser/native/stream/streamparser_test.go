package stream

import (
	"bytes"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestParseDropsEmptyTagKeys(t *testing.T) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f := func(metricName []byte, resultExpected string) {
		t.Helper()

		const timestamp = 1700000000000
		var b storage.Block
		b.Init(&storage.TSID{}, []int64{timestamp}, []int64{1}, 0, 64)
		block := b.MarshalPortable(nil)

		payload := encoding.MarshalInt64(nil, timestamp)
		payload = encoding.MarshalInt64(payload, timestamp)
		payload = encoding.MarshalUint32(payload, uint32(len(metricName)))
		payload = append(payload, metricName...)
		payload = encoding.MarshalUint32(payload, uint32(len(block)))
		payload = append(payload, block...)

		var mu sync.Mutex
		var results []string
		err := Parse(bytes.NewReader(payload), "", func(block *Block) error {
			mu.Lock()
			results = append(results, block.MetricName.String())
			mu.Unlock()
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if len(results) != 1 || results[0] != resultExpected {
			t.Fatalf("unexpected metric names; got %q; want [%q]", results, resultExpected)
		}
	}

	// Empty tag key must be dropped, since it overrides the metric name.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4962
	f([]byte("m\x01\x01v\x01"), "m{}")

	mn := &storage.MetricName{
		MetricGroup: []byte("m"),
		Tags: []storage.Tag{
			{Key: []byte("a"), Value: []byte("1")},
			{Key: []byte(""), Value: []byte("v")},
			{Key: []byte("b"), Value: []byte("2")},
		},
	}
	f(mn.Marshal(nil), `m{a="1",b="2"}`)
}
