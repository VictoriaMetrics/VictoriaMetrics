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
	pc := newParseCache()
	if pc.len() != 0 || pc.misses() != 0 || pc.requests() != 0 {
		t.Errorf("unexpected pc.Len()=%d, pc.Misses()=%d, pc.Requests()=%d; expected all to be zero.", pc.len(), pc.misses(), pc.requests())
	}

	q1 := `foo{bar="baz"}`
	v1 := testGetParseCacheValue(q1)

	q2 := `foo1{bar1="baz1"}`
	v2 := testGetParseCacheValue(q2)

	pc.put(q1, v1)
	if pc.len() != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", pc.len(), 1)
	}

	if res := pc.get(q2); res != nil {
		t.Errorf("unexpected non-empty value obtained from cache: %d ", res)
	}
	if pc.len() != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", pc.len(), 1)
	}
	if miss := pc.misses(); miss != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", miss, 1)
	}
	if req := pc.requests(); req != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", req, 1)
	}

	pc.put(q2, v2)
	if pc.len() != 2 {
		t.Errorf("unexpected value obtained; got %d; want %d", pc.len(), 2)
	}

	if res := pc.get(q1); res != v1 {
		t.Errorf("unexpected value obtained; got %v; want %v", res, v1)
	}

	if res := pc.get(q2); res != v2 {
		t.Errorf("unexpected value obtained; got %v; want %v", res, v2)
	}

	pc.put(q2, v2)
	if pc.len() != 2 {
		t.Errorf("unexpected value obtained; got %d; want %d", pc.len(), 2)
	}
	if miss := pc.misses(); miss != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", miss, 1)
	}
	if req := pc.requests(); req != 3 {
		t.Errorf("unexpected value obtained; got %d; want %d", req, 3)
	}

	if res := pc.get(q2); res != v2 {
		t.Errorf("unexpected value obtained; got %v; want %v", res, v2)
	}
	if pc.len() != 2 {
		t.Errorf("unexpected value obtained; got %d; want %d", pc.len(), 2)
	}
	if miss := pc.misses(); miss != 1 {
		t.Errorf("unexpected value obtained; got %d; want %d", miss, 1)
	}
	if req := pc.requests(); req != 4 {
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
		b.put(queries[i], v)
	}
	expectedLen = uint64(parseBucketMaxLen)
	if b.len() != expectedLen {
		t.Errorf("unexpected value obtained; got %v; want %v", b.len(), expectedLen)
	}

	// Overflow bucket
	expectedLen = uint64(parseBucketMaxLen + 1)
	b.put(queries[parseBucketMaxLen], v)
	if b.len() != uint64(expectedLen) {
		t.Errorf("unexpected value obtained; got %v; want %v", b.len(), expectedLen)
	}

	// Clean up;
	oldLen := b.len()
	overflow := int(float64(oldLen) * parseBucketFreePercent)
	expectedLen = oldLen - uint64(overflow) + 1 // +1 for new entry

	b.put(queries[parseBucketMaxLen+1], v)
	if b.len() != expectedLen {
		t.Errorf("unexpected value obtained; got %v; want %v", b.len(), expectedLen)
	}
}
