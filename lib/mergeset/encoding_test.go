package mergeset

import (
	"math/rand"
	"reflect"
	"sort"
	"sync"
	"testing"
	"testing/quick"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestInmemoryBlockAdd(t *testing.T) {
	var ib inmemoryBlock

	for i := 0; i < 30; i++ {
		var items []string
		totalLen := 0
		ib.Reset()

		// Fill ib.
		for j := 0; j < i*100+1; j++ {
			s := getRandomBytes()
			if !ib.Add(s) {
				// ib is full.
				break
			}
			items = append(items, string(s))
			totalLen += len(s)
		}

		// Verify all the items are added.
		if len(ib.items) != len(items) {
			t.Fatalf("unexpected number of items added; got %d; want %d", len(ib.items), len(items))
		}
		if len(ib.data) != totalLen {
			t.Fatalf("unexpected ib.data len; got %d; want %d", len(ib.data), totalLen)
		}
		for j, item := range ib.items {
			if items[j] != string(item) {
				t.Fatalf("unexpected item at index %d out of %d, loop %d\ngot\n%X\nwant\n%X", j, len(items), i, item, items[j])
			}
		}
	}
}

func TestInmemoryBlockSort(t *testing.T) {
	var ib inmemoryBlock

	for i := 0; i < 100; i++ {
		var items []string
		totalLen := 0
		ib.Reset()

		// Fill ib.
		for j := 0; j < rand.Intn(1500); j++ {
			s := getRandomBytes()
			if !ib.Add(s) {
				// ib is full.
				break
			}
			items = append(items, string(s))
			totalLen += len(s)
		}

		// Sort ib.
		ib.sort()
		sort.Strings(items)

		// Verify items are sorted.
		if len(ib.items) != len(items) {
			t.Fatalf("unexpected number of items added; got %d; want %d", len(ib.items), len(items))
		}
		if len(ib.data) != totalLen {
			t.Fatalf("unexpected ib.data len; got %d; want %d", len(ib.data), totalLen)
		}
		for j, item := range ib.items {
			if items[j] != string(item) {
				t.Fatalf("unexpected item at index %d out of %d, loop %d\ngot\n%X\nwant\n%X", j, len(items), i, item, items[j])
			}
		}
	}
}

func TestInmemoryBlockMarshalUnmarshal(t *testing.T) {
	var ib, ib2 inmemoryBlock
	var sb storageBlock
	var firstItem, commonPrefix []byte
	var itemsLen uint32
	var mt marshalType

	for i := 0; i < 1000; i++ {
		var items []string
		totalLen := 0
		ib.Reset()

		// Fill ib.
		itemsCount := 2 * (rand.Intn(i+1) + 1)
		for j := 0; j < itemsCount/2; j++ {
			s := getRandomBytes()
			s = []byte("prefix " + string(s))
			if !ib.Add(s) {
				// ib is full.
				break
			}
			items = append(items, string(s))
			totalLen += len(s)

			s = getRandomBytes()
			if !ib.Add(s) {
				// ib is full
				break
			}
			items = append(items, string(s))
			totalLen += len(s)
		}

		// Marshal ib.
		sort.Strings(items)
		firstItem, commonPrefix, itemsLen, mt = ib.MarshalUnsortedData(&sb, firstItem[:0], commonPrefix[:0], 0)
		if int(itemsLen) != len(ib.items) {
			t.Fatalf("unexpected number of items marshaled; got %d; want %d", itemsLen, len(ib.items))
		}
		if string(firstItem) != string(ib.items[0]) {
			t.Fatalf("unexpected the first item\ngot\n%q\nwant\n%q", firstItem, ib.items[0])
		}
		if err := checkMarshalType(mt); err != nil {
			t.Fatalf("invalid mt: %s", err)
		}

		// Unmarshal ib.
		if err := ib2.UnmarshalData(&sb, firstItem, commonPrefix, itemsLen, mt); err != nil {
			t.Fatalf("cannot unmarshal data for firstItem=%q, commonPrefix=%q, itemsLen=%d, mt=%d: %s",
				firstItem, commonPrefix, itemsLen, mt, err)
		}

		// Verify all the items are sorted and unmarshaled.
		if len(ib2.items) != len(items) {
			t.Fatalf("unexpected number of items unmarshaled; got %d; want %d", len(ib2.items), len(items))
		}
		if len(ib2.data) != totalLen {
			t.Fatalf("unexpected ib.data len; got %d; want %d", len(ib2.data), totalLen)
		}
		for j := range items {
			if len(items[j]) != len(ib2.items[j]) {
				t.Fatalf("items length mismatch at index %d out of %d, loop %d\ngot\n(len=%d) %X\nwant\n(len=%d) %X",
					j, len(items), i, len(ib2.items[j]), ib2.items[j], len(items[j]), items[j])
			}
		}
		for j, item := range ib2.items {
			if items[j] != string(item) {
				t.Fatalf("unexpected item at index %d out of %d, loop %d\ngot\n(len=%d) %X\nwant\n(len=%d) %X",
					j, len(items), i, len(item), item, len(items[j]), items[j])
			}
		}
	}
}

func getRandomBytes() []byte {
	rndLock.Lock()
	iv, ok := quick.Value(bytesType, rnd)
	rndLock.Unlock()
	if !ok {
		logger.Panicf("error in quick.Value when generating random string")
	}
	return iv.Interface().([]byte)
}

var bytesType = reflect.TypeOf([]byte(nil))

var (
	rnd     = rand.New(rand.NewSource(1))
	rndLock sync.Mutex
)
