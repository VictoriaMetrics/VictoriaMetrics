package storage

import (
	"fmt"
	"reflect"
	"testing"
	"testing/quick"
)

func TestMetaindexRowReset(t *testing.T) {
	var mr metaindexRow

	mr.TSID.MetricID = 234
	mr.BlockHeadersCount = 1323
	mr.MinTimestamp = -234
	mr.MaxTimestamp = 8989
	mr.IndexBlockOffset = 89439
	mr.IndexBlockSize = 89984

	var mrEmpty metaindexRow
	mrEmpty.MinTimestamp = 1<<63 - 1
	mrEmpty.MaxTimestamp = -1 << 63
	if reflect.DeepEqual(&mr, &mrEmpty) {
		t.Fatalf("mr=%+v cannot be equal to mrEmpty=%+v", &mr, &mrEmpty)
	}
	mr.Reset()
	if !reflect.DeepEqual(&mr, &mrEmpty) {
		t.Fatalf("mr=%+v must be equal to mrEmpty=%+v", &mr, &mrEmpty)
	}
}

func TestMetaindexRowMarshalUnmarshal(t *testing.T) {
	var mr metaindexRow

	for i := 0; i < 1000; i++ {
		initTestMetaindexRow(&mr)
		testMetaindexRowMarshalUnmarshal(t, &mr)
	}
}

func testMetaindexRowMarshalUnmarshal(t *testing.T, mr *metaindexRow) {
	dst := mr.Marshal(nil)
	var mr1 metaindexRow
	tail, err := mr1.Unmarshal(dst)
	if err != nil {
		t.Fatalf("cannot unmarshal mr=%+v from dst=%x: %s", mr, dst, err)
	}
	if len(tail) > 0 {
		t.Fatalf("unexpected non-zero tail got after unmarshaling mr=%+v from dst=%x: %x", mr, dst, tail)
	}
	if !reflect.DeepEqual(mr, &mr1) {
		t.Fatalf("unexpected unmarshaled mr; got\n%+v; want\n%+v", &mr1, mr)
	}

	prefix := []byte("foo")
	dstNew := mr.Marshal(prefix)
	if string(dstNew[:len(prefix)]) != string(prefix) {
		t.Fatalf("unexepcted prefix when marshaling mr=%+v; got\n%x; want\n%x", mr, dstNew[:len(prefix)], prefix)
	}
	if string(dstNew[len(prefix):]) != string(dst) {
		t.Fatalf("unexpected prefixed dstNew for mr=%+v; got\n%x; want\n%x", mr, dstNew[len(prefix):], dst)
	}

	suffix := []byte("bar")
	dst = append(dst, suffix...)
	var mr2 metaindexRow
	tail, err = mr2.Unmarshal(dst)
	if err != nil {
		t.Fatalf("cannot unmarshal mr=%+v from prefixed dst=%x: %s", mr, dst, err)
	}
	if string(tail) != string(suffix) {
		t.Fatalf("invalid tail after unmarshaling mr=%+v from prefixed dst; got\n%x; want\n%x", mr, tail, suffix)
	}
	if !reflect.DeepEqual(mr, &mr2) {
		t.Fatalf("unexpected unmarshaled mr from prefixed dst; got\n%+v; want\n%+v", &mr2, mr)
	}
}

func initTestMetaindexRow(mr *metaindexRow) {
	rndLock.Lock()
	iv, ok := quick.Value(metaindexRowType, rnd)
	rndLock.Unlock()
	if !ok {
		panic(fmt.Errorf("error in quick.Value when generating random metaindexRow"))
	}
	rndMR := iv.Interface().(*metaindexRow)
	if rndMR == nil {
		rndMR = &metaindexRow{}
	}
	*mr = *rndMR
	if mr.BlockHeadersCount == 0 {
		mr.BlockHeadersCount = 1
	}
	if mr.IndexBlockSize > 2*8*maxBlockSize {
		mr.IndexBlockSize = 2 * 8 * maxBlockSize
	}
}

var metaindexRowType = reflect.TypeOf(&metaindexRow{})
