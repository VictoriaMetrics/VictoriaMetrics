package netstorage

import (
	"bytes"
	"flag"
	"math"
	"net"
	"reflect"
	"runtime"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/handshake"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/vmselectapi"
)

func TestInitStopNodes(t *testing.T) {
	if err := flag.Set("vmstorageDialTimeout", "1ms"); err != nil {
		t.Fatalf("cannot set vmstorageDialTimeout flag: %s", err)
	}
	for range 3 {
		Init([]string{"host1", "host2"})
		runtime.Gosched()
		MustStop()
	}

	// Try initializing the netstorage with bigger number of nodes
	for range 3 {
		Init([]string{"host1", "host2", "host3"})
		runtime.Gosched()
		MustStop()
	}

	// Try initializing the netstorage with smaller number of nodes
	for range 3 {
		Init([]string{"host1"})
		runtime.Gosched()
		MustStop()
	}
}

func TestMergeSortBlocks(t *testing.T) {
	f := func(blocks []*sortBlock, dedupInterval int64, expectedResult *Result) {
		t.Helper()
		var result Result
		sbh := getSortBlocksHeap()
		sbh.sbs = append(sbh.sbs[:0], blocks...)
		mergeSortBlocks(&result, sbh, dedupInterval)
		putSortBlocksHeap(sbh)
		if !reflect.DeepEqual(result.Values, expectedResult.Values) {
			t.Fatalf("unexpected values;\ngot\n%v\nwant\n%v", result.Values, expectedResult.Values)
		}
		if !reflect.DeepEqual(result.Timestamps, expectedResult.Timestamps) {
			t.Fatalf("unexpected timestamps;\ngot\n%v\nwant\n%v", result.Timestamps, expectedResult.Timestamps)
		}
	}

	// Zero blocks
	f(nil, 1, &Result{})

	// Single block without samples
	f([]*sortBlock{{}}, 1, &Result{})

	// Single block with a single samples.
	f([]*sortBlock{
		{
			Timestamps: []int64{1},
			Values:     []float64{4.2},
		},
	}, 1, &Result{
		Timestamps: []int64{1},
		Values:     []float64{4.2},
	})

	// Single block with multiple samples.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 3},
			Values:     []float64{4.2, 2.1, 10},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 3},
		Values:     []float64{4.2, 2.1, 10},
	})

	// Single block with multiple samples with deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 3},
			Values:     []float64{4.2, 2.1, 10},
		},
	}, 2, &Result{
		Timestamps: []int64{2, 3},
		Values:     []float64{2.1, 10},
	})

	// Multiple blocks without time range intersection.
	f([]*sortBlock{
		{
			Timestamps: []int64{3, 5},
			Values:     []float64{5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2},
			Values:     []float64{4.2, 2.1},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 3, 5},
		Values:     []float64{4.2, 2.1, 5.2, 6.1},
	})

	// Multiple blocks with time range intersection.
	f([]*sortBlock{
		{
			Timestamps: []int64{3, 5},
			Values:     []float64{5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 3, 4, 5},
		Values:     []float64{4.2, 2.1, 5.2, 42, 6.1},
	})

	// Multiple blocks with time range inclusion.
	f([]*sortBlock{
		{
			Timestamps: []int64{0, 3, 5},
			Values:     []float64{9, 5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 1, &Result{
		Timestamps: []int64{0, 1, 2, 3, 4, 5},
		Values:     []float64{9, 4.2, 2.1, 5.2, 42, 6.1},
	})

	// Multiple blocks with identical timestamps and identical values.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 4, 5},
			Values:     []float64{9, 5.2, 6.1, 9},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{9, 5.2, 6.1},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 4, 5},
		Values:     []float64{9, 5.2, 6.1, 9},
	})

	// Multiple blocks with identical timestamps.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 4, 5},
			Values:     []float64{9, 5.2, 6.1, 9},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 4, 5},
		Values:     []float64{9, 5.2, 42, 9},
	})
	// Multiple blocks with identical timestamps, disabled deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{9, 5.2, 6.1},
		},
		{
			Timestamps: []int64{1, 2, 4},
			Values:     []float64{4.2, 2.1, 42},
		},
	}, 0, &Result{
		Timestamps: []int64{1, 1, 2, 2, 4, 4},
		Values:     []float64{9, 4.2, 2.1, 5.2, 6.1, 42},
	})

	// Multiple blocks with identical timestamp ranges.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 5, 10, 11},
			Values:     []float64{9, 8, 7, 6, 5},
		},
		{
			Timestamps: []int64{1, 2, 4, 10, 11, 12},
			Values:     []float64{21, 22, 23, 24, 25, 26},
		},
	}, 1, &Result{
		Timestamps: []int64{1, 2, 4, 5, 10, 11, 12},
		Values:     []float64{21, 22, 23, 7, 24, 25, 26},
	})

	// Multiple blocks with identical timestamp ranges, no deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 5, 10, 11},
			Values:     []float64{9, 8, 7, 6, 5},
		},
		{
			Timestamps: []int64{1, 2, 4, 10, 11, 12},
			Values:     []float64{21, 22, 23, 24, 25, 26},
		},
	}, 0, &Result{
		Timestamps: []int64{1, 1, 2, 2, 4, 5, 10, 10, 11, 11, 12},
		Values:     []float64{9, 21, 22, 8, 23, 7, 6, 24, 25, 5, 26},
	})

	// Multiple blocks with identical timestamp ranges with deduplication.
	f([]*sortBlock{
		{
			Timestamps: []int64{1, 2, 5, 10, 11},
			Values:     []float64{9, 8, 7, 6, 5},
		},
		{
			Timestamps: []int64{1, 2, 4, 10, 11, 12},
			Values:     []float64{21, 22, 23, 24, 25, 26},
		},
	}, 5, &Result{
		Timestamps: []int64{5, 10, 12},
		Values:     []float64{7, 24, 26},
	})
}

func TestEqualSamplesPrefix(t *testing.T) {
	f := func(a, b *sortBlock, expected int) {
		t.Helper()

		actual := equalSamplesPrefix(a, b)
		if actual != expected {
			t.Fatalf("unexpected result: got %d, want %d", actual, expected)
		}
	}

	// Empty blocks
	f(&sortBlock{}, &sortBlock{}, 0)

	// Identical blocks
	f(&sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, 6, 7, 8},
	}, &sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, 6, 7, 8},
	}, 4)

	// Non-zero NextIdx
	f(&sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, 6, 7, 8},
		NextIdx:    2,
	}, &sortBlock{
		Timestamps: []int64{10, 20, 3, 4},
		Values:     []float64{50, 60, 7, 8},
		NextIdx:    2,
	}, 2)

	// Non-zero NextIdx with mismatch
	f(&sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, 6, 7, 8},
		NextIdx:    1,
	}, &sortBlock{
		Timestamps: []int64{10, 2, 3, 4},
		Values:     []float64{50, 6, 7, 80},
		NextIdx:    1,
	}, 2)

	// Different lengths
	f(&sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, 6, 7, 8},
	}, &sortBlock{
		Timestamps: []int64{1, 2, 3},
		Values:     []float64{5, 6, 7},
	}, 3)

	// Timestamps diverge
	f(&sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, 6, 7, 8},
	}, &sortBlock{
		Timestamps: []int64{1, 2, 30, 4},
		Values:     []float64{5, 6, 7, 8},
	}, 2)

	// Values diverge
	f(&sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, 6, 7, 8},
	}, &sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, 60, 7, 8},
	}, 1)

	// Zero matches
	f(&sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, 6, 7, 8},
	}, &sortBlock{
		Timestamps: []int64{5, 6, 7, 8},
		Values:     []float64{1, 2, 3, 4},
	}, 0)

	// Compare staleness markers, matching
	f(&sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, decimal.StaleNaN, 7, 8},
	}, &sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{5, decimal.StaleNaN, 7, 8},
	}, 4)

	// Special float values: +Inf, -Inf, 0, -0
	f(&sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{math.Inf(1), math.Inf(-1), math.Copysign(0, +1), math.Copysign(0, -1)},
	}, &sortBlock{
		Timestamps: []int64{1, 2, 3, 4},
		Values:     []float64{math.Inf(1), math.Inf(-1), math.Copysign(0, +1), math.Copysign(0, -1)},
	}, 4)

	// Positive zero vs negative zero (bitwise different)
	f(&sortBlock{
		Timestamps: []int64{1, 2},
		Values:     []float64{5, math.Copysign(0, +1)},
	}, &sortBlock{
		Timestamps: []int64{1, 2},
		Values:     []float64{5, math.Copysign(0, -1)},
	}, 1)
}

func TestReadBytesWithMetadata(t *testing.T) {
	f := func(rawPayload []byte, maxDataSize int, expectedData []byte, expectedMeta BlockMetadata, expectErr bool) {
		t.Helper()

		// Use net.Pipe to simulate a network connection
		clientConn, serverConn := net.Pipe()

		go func() {
			defer clientConn.Close()
			// Must use client handshake to properly initialize BufferedConn
			bcClient, err := handshake.VMSelectClient(clientConn, 0)
			if err == nil {
				// Send the constructed binary payload to the receiver
				bcClient.Write(rawPayload)
				bcClient.Flush()
			}
		}()

		// Simulate the receiver side
		bcServer, err := handshake.VMSelectServer(serverConn, 0)
		if err != nil {
			t.Fatalf("unexpected handshake error: %v", err)
		}
		defer serverConn.Close()

		// Invoke the function under test
		buf := make([]byte, 0)
		resultData, meta, err := readBytesWithMetadata(buf, bcServer, maxDataSize)

		// Validate error behavior
		if expectErr {
			if err == nil {
				t.Fatal("expected error but got nil")
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Validate BlockMetadata parsing results
		if meta.IsMetadata != expectedMeta.IsMetadata {
			t.Errorf("unexpected IsMetadata: got %v, want %v", meta.IsMetadata, expectedMeta.IsMetadata)
		}
		if meta.Version != expectedMeta.Version {
			t.Errorf("unexpected Version: got %d, want %d", meta.Version, expectedMeta.Version)
		}
		if meta.FieldIndex != expectedMeta.FieldIndex {
			t.Errorf("unexpected FieldIndex: got %d, want %d", meta.FieldIndex, expectedMeta.FieldIndex)
		}

		// Validate data payload
		if !bytes.Equal(resultData, expectedData) {
			t.Errorf("unexpected data: got %q, want %q", resultData, expectedData)
		}
	}

	// Helper function to construct the protocol header
	// Based on logic: rawHeader = (Version << 61) | (FieldIndex << 48) | FlagMetadata | DataSize
	buildHeader := func(isMeta bool, version uint8, fieldIndex uint16, dataSize uint64) uint64 {
		header := dataSize
		if isMeta {
			header |= vmselectapi.FlagMetadata
			header |= (uint64(version) & uint64(vmselectapi.MaskVersion)) << 61
			header |= (uint64(fieldIndex) & uint64(vmselectapi.MaskFieldIndex)) << 48
		}
		return header
	}

	buildPayload := func(header uint64, data []byte) []byte {
		p := encoding.MarshalUint64(nil, header)
		p = append(p, data...)
		return p
	}

	// --- Test Cases ---

	// 1. Normal data block (IsMetadata = false)
	{
		data := []byte("plain data")
		header := buildHeader(false, 0, 0, uint64(len(data)))
		expectedMeta := BlockMetadata{IsMetadata: false}
		f(buildPayload(header, data), 1024, data, expectedMeta, false)
	}

	// 2. Metadata block - contains Version and FieldIndex (e.g., isPartial flag)
	{
		data := []byte{1} // Assume payload is boolean true
		version := uint8(1)
		fieldIndex := uint16(vmselectapi.FieldIndexIsPartial)
		header := buildHeader(true, version, fieldIndex, uint64(len(data)))
		expectedMeta := BlockMetadata{
			IsMetadata: true,
			Version:    version,
			FieldIndex: fieldIndex,
		}
		f(buildPayload(header, data), 1024, data, expectedMeta, false)
	}

	// 3. Boundary case: data size is exactly equal to maxDataSize
	{
		data := bytes.Repeat([]byte("a"), 100)
		header := buildHeader(false, 0, 0, 100)
		f(buildPayload(header, data), 100, data, BlockMetadata{IsMetadata: false}, false)
	}

	// 4. Error case: data size exceeds maxDataSize
	{
		header := buildHeader(true, 1, 2, 500)
		f(buildPayload(header, make([]byte, 500)), 100, nil, BlockMetadata{}, true)
	}

	// 5. Error case: empty payload error (Length declared as 10, but connection closes)
	{
		header := buildHeader(false, 0, 0, 10)
		f(encoding.MarshalUint64(nil, header), 1024, nil, BlockMetadata{}, true)
	}

	// 6. Bitmask validation for Version and FieldIndex (testing overflow truncation)
	{
		// Assume Version only has 3 bits (MaskVersion = 0x7)
		data := []byte("meta")
		header := buildHeader(true, 0x9, 0, uint64(len(data))) // 0x9 should be truncated to 0x1 by the mask
		expectedMeta := BlockMetadata{
			IsMetadata: true,
			Version:    0x1, // 0x9 & 0x7
			FieldIndex: 0,
		}
		f(buildPayload(header, data), 1024, data, expectedMeta, false)
	}
}
