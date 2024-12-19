package promql

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
)

func testGetParseCacheValue(q string) *parseCacheValue {
	e, err := metricsql.Parse(q)
	return &parseCacheValue{
		e:   e,
		err: err,
	}
}

func testGenerateQueries(items int) []string {
	queries := make([]string, items)
	for i := 0; i < items; i++ {
		queries[i] = fmt.Sprintf(`node_time_seconds{instance="node%d", job="job%d"}`, i, i)
	}
	return queries
}

func TestParseCache(t *testing.T) {
	pc := NewParseCache()
	if pc.Len() != 0 || pc.Misses() != 0 || pc.Requests() != 0 {
		t.Errorf("unexpected pc.Len()=%d, pc.Misses()=%d, pc.Requests()=%d; expected all to be zero.", pc.Len(), pc.Misses(), pc.Requests())
	}

	q1 := `foo{bar="baz"}`
	v1 := testGetParseCacheValue(q1)

	q2 := `foo1{bar1="baz1"}`
	v2 := testGetParseCacheValue(q2)

	pc.Put(q1, v1)
	if len := pc.Len(); len != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", len, 1)
	}

	if res := pc.Get(q2); res != nil {
		t.Errorf("unexpected non-empty value obtained from cache: %d ", res)
	}
	if len := pc.Len(); len != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", len, 1)
	}
	if miss := pc.Misses(); miss != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", miss, 1)
	}
	if req := pc.Requests(); req != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", req, 1)
	}

	pc.Put(q2, v2)
	if len := pc.Len(); len != 2 {
		t.Errorf("unexpected value obtained; got %d; want %d", len, 2)
	}

	if res := pc.Get(q1); res != v1 {
		t.Errorf("unexpected value obtained; got %v; want %v", res, v1)
	}

	if res := pc.Get(q2); res != v2 {
		t.Errorf("unexpected value obtained; got %v; want %v", res, v2)
	}

	pc.Put(q2, v2)
	if len := pc.Len(); len != 2 {
		t.Errorf("unexpected value obtained; got %d; want %d", len, 2)
	}
	if miss := pc.Misses(); miss != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", miss, 1)
	}
	if req := pc.Requests(); req != 3 {
		t.Errorf("unexpected value obtained; got %d; want %d", req, 3)
	}

	if res := pc.Get(q2); res != v2 {
		t.Errorf("unexpected value obtained; got %v; want %v", res, v2)
	}
	if len := pc.Len(); len != 2 {
		t.Errorf("unexpected value obtained; got %d; want %d", len, 2)
	}
	if miss := pc.Misses(); miss != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", miss, 1)
	}
	if req := pc.Requests(); req != 4 {
		t.Errorf("unexpected value obtained; got %d; want %d", req, 4)
	}
}

func TestParseCacheBucketOverflow(t *testing.T) {
	b := newParseBucket()
	var expectedLen uint64

	// +2 for overflow and clean up
	queries := testGenerateQueries(parseBucketMaxLen + 2)

	// Same value for all keys
	v := testGetParseCacheValue(queries[0])

	// Fill bucket
	for i := 0; i < parseBucketMaxLen; i++ {
		b.Put(queries[i], v)
	}
	expectedLen = uint64(parseBucketMaxLen)
	if len := b.Len(); len != expectedLen {
		t.Errorf("unexpected value obtained; got %v; want %v", len, expectedLen)
	}

	// Overflow bucket
	expectedLen = uint64(parseBucketMaxLen + 1)
	b.Put(queries[parseBucketMaxLen], v)
	if len := b.Len(); len != uint64(expectedLen) {
		t.Errorf("unexpected value obtained; got %v; want %v", len, expectedLen)
	}

	// Clean up;
	oldLen := b.Len()
	overflow := int(float64(oldLen) * parseBucketFreePercent)
	expectedLen = oldLen - uint64(overflow) + 1 // +1 for new entry

	b.Put(queries[parseBucketMaxLen+1], v)
	if len := b.Len(); len != expectedLen {
		t.Errorf("unexpected value obtained; got %v; want %v", len, expectedLen)
	}
}
