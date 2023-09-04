package mergeset

import (
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"testing/quick"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func TestCommonPrefixLen(t *testing.T) {
	f := func(a, b string, expectedPrefixLen int) {
		t.Helper()
		prefixLen := commonPrefixLen([]byte(a), []byte(b))
		if prefixLen != expectedPrefixLen {
			t.Fatalf("unexpected prefix len; got %d; want %d", prefixLen, expectedPrefixLen)
		}
	}
	f("", "", 0)
	f("a", "", 0)
	f("", "a", 0)
	f("a", "a", 1)
	f("abc", "xy", 0)
	f("abc", "abd", 2)
	f("01234567", "01234567", 8)
	f("01234567", "012345678", 8)
	f("012345679", "012345678", 8)
	f("01234569", "012345678", 7)
	f("01234569", "01234568", 7)
}

func TestInmemoryBlockAdd(t *testing.T) {
	r := rand.New(rand.NewSource(1))

	var ib inmemoryBlock

	for i := 0; i < 30; i++ {
		var items []string
		totalLen := 0
		ib.Reset()

		// Fill ib.
		for j := 0; j < i*100+1; j++ {
			s := getRandomBytes(r)
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
		data := ib.data
		for j, it := range ib.items {
			item := it.String(data)
			if items[j] != item {
				t.Fatalf("unexpected item at index %d out of %d, loop %d\ngot\n%X\nwant\n%X", j, len(items), i, item, items[j])
			}
		}
	}
}

func TestInmemoryBlockSort(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	var ib inmemoryBlock

	for i := 0; i < 100; i++ {
		var items []string
		totalLen := 0
		ib.Reset()

		// Fill ib.
		for j := 0; j < r.Intn(1500); j++ {
			s := getRandomBytes(r)
			if !ib.Add(s) {
				// ib is full.
				break
			}
			items = append(items, string(s))
			totalLen += len(s)
		}

		// Sort ib.
		sort.Sort(&ib)
		sort.Strings(items)

		// Verify items are sorted.
		if len(ib.items) != len(items) {
			t.Fatalf("unexpected number of items added; got %d; want %d", len(ib.items), len(items))
		}
		if len(ib.data) != totalLen {
			t.Fatalf("unexpected ib.data len; got %d; want %d", len(ib.data), totalLen)
		}
		data := ib.data
		for j, it := range ib.items {
			item := it.String(data)
			if items[j] != item {
				t.Fatalf("unexpected item at index %d out of %d, loop %d\ngot\n%X\nwant\n%X", j, len(items), i, item, items[j])
			}
		}
	}
}

func TestInmemoryBlockMarshalUnmarshal(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	var ib, ib2 inmemoryBlock
	var sb storageBlock
	var firstItem, commonPrefix []byte
	var itemsLen uint32
	var mt marshalType

	for i := 0; i < 1000; i += 10 {
		var items []string
		totalLen := 0
		ib.Reset()

		// Fill ib.
		itemsCount := 2 * (r.Intn(i+1) + 1)
		for j := 0; j < itemsCount/2; j++ {
			s := getRandomBytes(r)
			s = []byte("prefix " + string(s))
			if !ib.Add(s) {
				// ib is full.
				break
			}
			items = append(items, string(s))
			totalLen += len(s)

			s = getRandomBytes(r)
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
		firstItemExpected := ib.items[0].String(ib.data)
		if string(firstItem) != firstItemExpected {
			t.Fatalf("unexpected the first item\ngot\n%q\nwant\n%q", firstItem, firstItemExpected)
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
			it2 := ib2.items[j]
			item2 := it2.String(ib2.data)
			if len(items[j]) != len(item2) {
				t.Fatalf("items length mismatch at index %d out of %d, loop %d\ngot\n(len=%d) %X\nwant\n(len=%d) %X",
					j, len(items), i, len(item2), item2, len(items[j]), items[j])
			}
		}
		for j, it := range ib2.items {
			item := it.String(ib2.data)
			if items[j] != string(item) {
				t.Fatalf("unexpected item at index %d out of %d, loop %d\ngot\n(len=%d) %X\nwant\n(len=%d) %X",
					j, len(items), i, len(item), item, len(items[j]), items[j])
			}
		}
	}
}

func getRandomBytes(r *rand.Rand) []byte {
	iv, ok := quick.Value(bytesType, r)
	if !ok {
		logger.Panicf("error in quick.Value when generating random string")
	}
	return iv.Interface().([]byte)
}

var bytesType = reflect.TypeOf([]byte(nil))
