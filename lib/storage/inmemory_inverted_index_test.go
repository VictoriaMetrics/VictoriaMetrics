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
	for i := 0; i < 10; i++ {
		var e pendingHourMetricIDEntry
		e.AccountID = uint32(i)
		e.ProjectID = uint32(i + 324)
		e.MetricID = uint64(i * 43)
		iidx.pendingEntries = append(iidx.pendingEntries, e)
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
	if !reflect.DeepEqual(iidx.pendingEntries, iidx2.pendingEntries) {
		t.Fatalf("unexpected pendingMetricIDs; got\n%v;\nwant\n%v", iidx2.pendingEntries, iidx.pendingEntries)
	}
	for k, v := range iidx.m {
		v2 := iidx2.m[k]
		if !v.Equal(v2) {
			t.Fatalf("unexpected set for key %q", k)
		}
	}
}
