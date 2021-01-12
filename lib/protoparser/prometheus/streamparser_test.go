package prometheus

import (
	"bytes"
	"compress/gzip"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
)

func TestParseStream(t *testing.T) {
	common.StartUnmarshalWorkers()
	defer common.StopUnmarshalWorkers()

	const defaultTimestamp = 123
	f := func(s string, rowsExpected []Row) {
		t.Helper()
		bb := bytes.NewBufferString(s)
		var result []Row
		var lock sync.Mutex
		doneCh := make(chan struct{})
		err := ParseStream(bb, defaultTimestamp, false, func(rows []Row) error {
			lock.Lock()
			result = appendRowCopies(result, rows)
			if len(result) == len(rowsExpected) {
				close(doneCh)
			}
			lock.Unlock()
			return nil
		}, nil)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		select {
		case <-doneCh:
		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
		sortRows(result)
		if !reflect.DeepEqual(result, rowsExpected) {
			t.Fatalf("unexpected rows parsed; got\n%v\nwant\n%v", result, rowsExpected)
		}

		// Parse compressed stream.
		bb.Reset()
		zw := gzip.NewWriter(bb)
		if _, err := zw.Write([]byte(s)); err != nil {
			t.Fatalf("unexpected error when gzipping %q: %s", s, err)
		}
		if err := zw.Close(); err != nil {
			t.Fatalf("unexpected error when closing gzip writer: %s", err)
		}
		result = nil
		doneCh = make(chan struct{})
		err = ParseStream(bb, defaultTimestamp, true, func(rows []Row) error {
			lock.Lock()
			result = appendRowCopies(result, rows)
			if len(result) == len(rowsExpected) {
				close(doneCh)
			}
			lock.Unlock()
			return nil
		}, nil)
		if err != nil {
			t.Fatalf("unexpected error when parsing compressed %q: %s", s, err)
		}
		select {
		case <-doneCh:
		case <-time.After(time.Second):
			t.Fatalf("timeout on compressed stream")
		}
		sortRows(result)
		if !reflect.DeepEqual(result, rowsExpected) {
			t.Fatalf("unexpected compressed rows parsed; got\n%v\nwant\n%v", result, rowsExpected)
		}
	}

	f("foo 123 456", []Row{{
		Metric:    "foo",
		Value:     123,
		Timestamp: 456000,
	}})
	f(`foo{bar="baz"} 1 2`+"\n"+`aaa{} 3 4`, []Row{
		{
			Metric:    "aaa",
			Value:     3,
			Timestamp: 4000,
		},
		{
			Metric: "foo",
			Tags: []Tag{{
				Key:   "bar",
				Value: "baz",
			}},
			Value:     1,
			Timestamp: 2000,
		},
	})
	f("foo 23", []Row{{
		Metric:    "foo",
		Value:     23,
		Timestamp: defaultTimestamp,
	}})
}

func sortRows(rows []Row) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		return a.Metric < b.Metric
	})
}

func appendRowCopies(dst, src []Row) []Row {
	for _, r := range src {
		// Make a copy of r, since r may contain garbage after returning from the callback to ParseStream.
		var rCopy Row
		rCopy.Metric = copyString(r.Metric)
		rCopy.Value = r.Value
		rCopy.Timestamp = r.Timestamp
		for _, tag := range r.Tags {
			rCopy.Tags = append(rCopy.Tags, Tag{
				Key:   copyString(tag.Key),
				Value: copyString(tag.Value),
			})
		}
		dst = append(dst, rCopy)
	}
	return dst
}

func copyString(s string) string {
	return string(append([]byte(nil), s...))
}
