package storage

import (
	"fmt"
	"reflect"
	"testing"
)

func TestInmemoryInvertedIndexMarshalUnmarshal(t *testing.T) {
	iidx := newInmemoryInvertedIndex()
	const keysCount = 100
	const metricIDsCount = 10000
	for i := 0; i < metricIDsCount; i++ {
		k := fmt.Sprintf("key %d", i%keysCount)
		iidx.addMetricIDLocked([]byte(k), uint64(i))
	}

	data := iidx.Marshal(nil)

	iidx2 := newInmemoryInvertedIndex()
	tail, err := iidx2.Unmarshal(data)
	if err != nil {
		t.Fatalf("cannot unmarshal iidx: %s", err)
	}
	if len(tail) != 0 {
		t.Fatalf("unexpected tail left after iidx unmarshaling: %d bytes", len(tail))
	}
	if len(iidx.m) != len(iidx2.m) {
		t.Fatalf("unexpected len(iidx2.m); got %d; want %d", len(iidx2.m), len(iidx.m))
	}
	if !reflect.DeepEqual(iidx.pendingMetricIDs, iidx2.pendingMetricIDs) {
		t.Fatalf("unexpected pendingMetricIDs; got\n%d;\nwant\n%d", iidx2.pendingMetricIDs, iidx.pendingMetricIDs)
	}
	for k, v := range iidx.m {
		v2 := iidx2.m[k]
		if !v.Equal(v2) {
			t.Fatalf("unexpected set for key %q", k)
		}
	}
}
