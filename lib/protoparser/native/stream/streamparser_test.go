package stream

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestParseRejectsEmptyTagKeys(t *testing.T) {
	protoparserutil.StartUnmarshalWorkers()
	defer protoparserutil.StopUnmarshalWorkers()

	f := func(metricName []byte, resultExpected, errExpected string) {
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
		if errExpected != "" {
			if err == nil || !strings.Contains(err.Error(), errExpected) {
				t.Fatalf("unexpected error; want error containing %q, got %v", errExpected, err)
			}
			if len(results) != 0 {
				t.Fatalf("callback called for invalid metric name: %q", results)
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if len(results) != 1 || results[0] != resultExpected {
			t.Fatalf("unexpected metric names; got %q; want [%q]", results, resultExpected)
		}
	}

	// An empty tag key would override the metric name at ingestion.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4962
	mn := storage.MetricName{
		MetricGroup: []byte("m"),
		Tags: []storage.Tag{
			{Key: []byte(""), Value: []byte("v")},
		},
	}
	f(mn.Marshal(nil), "", "invalid metricName: tag name cannot be empty")

	mn.Tags = []storage.Tag{
		{Key: []byte("a"), Value: []byte("1")},
		{Key: []byte(""), Value: []byte("v")},
		{Key: []byte("b"), Value: []byte("2")},
	}
	f(mn.Marshal(nil), "", "invalid metricName: tag name cannot be empty")

	mn.Tags = []storage.Tag{
		{Key: []byte("a"), Value: []byte("1")},
		{Key: []byte("b"), Value: []byte("2")},
	}
	f(mn.Marshal(nil), `m{a="1",b="2"}`, "")
}
