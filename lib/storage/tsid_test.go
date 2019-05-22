package storage

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"testing"
	"testing/quick"
)

func TestMarshaledTSIDSize(t *testing.T) {
	// This test makes sure marshaled format isn't changed.
	// If this test breaks then the storage format has been changed,
	// so it may become incompatible with the previously written data.
	expectedSize := 24
	if marshaledTSIDSize != expectedSize {
		t.Fatalf("unexpected marshaledTSIDSize; got %d; want %d", marshaledTSIDSize, expectedSize)
	}
}

func TestTSIDLess(t *testing.T) {
	var t1, t2 TSID
	if t1.Less(&t2) {
		t.Fatalf("t1=%v cannot be less than t2=%v", &t1, &t2)
	}
	if t2.Less(&t1) {
		t.Fatalf("t2=%v cannot be less than t1=%v", &t2, &t1)
	}

	t1.MetricID = 124
	t2.MetricID = 126
	t1.MetricGroupID = 847
	if t1.Less(&t2) {
		t.Fatalf("t1=%v cannot be less than t2=%v", &t1, &t2)
	}
	if !t2.Less(&t1) {
		t.Fatalf("t2=%v must be less than t1=%v", &t2, &t1)
	}

	t2 = t1
	t2.MetricID = 123
	t1.JobID = 84
	if t1.Less(&t2) {
		t.Fatalf("t1=%v cannot be less than t2=%v", &t1, &t2)
	}
	if !t2.Less(&t1) {
		t.Fatalf("t2=%v must be less than t1=%v", &t2, &t1)
	}

	t2 = t1
	t2.MetricID = 123
	t1.InstanceID = 8478
	if t1.Less(&t2) {
		t.Fatalf("t1=%v cannot be less than t2=%v", &t1, &t2)
	}
	if !t2.Less(&t1) {
		t.Fatalf("t2=%v must be less than t1=%v", &t2, &t1)
	}

	t2 = t1
	t1.MetricID = 123847
	if t1.Less(&t2) {
		t.Fatalf("t1=%v cannot be less than t2=%v", &t1, &t2)
	}
	if !t2.Less(&t1) {
		t.Fatalf("t2=%v must be less than t1=%v", &t2, &t1)
	}

	t2 = t1
	if t1.Less(&t2) {
		t.Fatalf("t1=%v cannot be less than t2=%v", &t1, &t2)
	}
	if t2.Less(&t1) {
		t.Fatalf("t2=%v cannot be less than t1=%v", &t2, &t1)
	}
}

func TestTSIDMarshalUnmarshal(t *testing.T) {
	var tsid TSID
	testTSIDMarshalUnmarshal(t, &tsid)

	for i := 0; i < 1000; i++ {
		initTestTSID(&tsid)

		testTSIDMarshalUnmarshal(t, &tsid)
	}
}

func initTestTSID(tsid *TSID) {
	rndLock.Lock()
	iv, ok := quick.Value(tsidType, rnd)
	rndLock.Unlock()
	if !ok {
		panic(fmt.Errorf("error in quick.Value when generating random TSID"))
	}
	rndTSID := iv.Interface().(*TSID)
	if rndTSID == nil {
		rndTSID = &TSID{}
	}
	*tsid = *rndTSID
}

var tsidType = reflect.TypeOf(&TSID{})

var (
	rnd     = rand.New(rand.NewSource(1))
	rndLock sync.Mutex
)

func testTSIDMarshalUnmarshal(t *testing.T, tsid *TSID) {
	t.Helper()

	dst := tsid.Marshal(nil)
	if len(dst) != marshaledTSIDSize {
		t.Fatalf("unexpected marshaled TSID size; got %d; want %d", len(dst), marshaledTSIDSize)
	}
	var tsid1 TSID
	tail, err := tsid1.Unmarshal(dst)
	if err != nil {
		t.Fatalf("cannot unmarshal tsid from dst=%x: %s", dst, err)
	}
	if len(tail) > 0 {
		t.Fatalf("non-zero tail left after unmarshaling tsid from dst=%x; %x", dst, tail)
	}
	if *tsid != tsid1 {
		t.Fatalf("unexpected tsid unmarshaled; got\n%+v; want\n%+v", &tsid1, tsid)
	}

	prefix := []byte("foo")
	dstNew := tsid.Marshal(prefix)
	if string(dstNew[:len(prefix)]) != string(prefix) {
		t.Fatalf("unexpected prefix: got\n%x; want\n%x", dstNew[:len(prefix)], prefix)
	}
	if string(dstNew[len(prefix):]) != string(dst) {
		t.Fatalf("unexpected prefixed dstNew; got\n%x; want\n%x", dstNew[len(prefix):], dst)
	}

	suffix := []byte("bar")
	dst = append(dst, suffix...)
	var tsid2 TSID
	tail, err = tsid2.Unmarshal(dst)
	if err != nil {
		t.Fatalf("cannot unmarshal tsid from suffixed dst=%x: %s", dst, err)
	}
	if string(tail) != string(suffix) {
		t.Fatalf("invalid suffix; got\n%x; want\n%x", tail, suffix)
	}
	if *tsid != tsid2 {
		t.Fatalf("unexpected tsid unmarshaled from suffixed dst; got\n%+v; want\n%+v", &tsid2, tsid)
	}
}
