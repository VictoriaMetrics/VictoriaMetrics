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
	// Helper closure for test assertions
	f := func(rawPayload []byte, maxDataSize int, expectedData []byte, expectedIsMeta bool, expectErr bool) {
		t.Helper()

		// Use net.Pipe() to simulate an in-memory full-duplex network connection
		clientConn, serverConn := net.Pipe()

		// Start a background goroutine to simulate the sender (Local vmselect / vmstorage)
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

		// Simulate the receiver (Global vmselect / Local vmselect)
		bcServer, err := handshake.VMSelectServer(serverConn, 0)
		if err != nil {
			t.Fatalf("unexpected handshake error: %v", err)
		}
		defer serverConn.Close()

		// Invoke the function under test
		buf := make([]byte, 0)
		resultData, isMeta, err := readBytesWithMetadata(buf, bcServer, maxDataSize)

		// Validate expected error behavior
		if expectErr {
			if err == nil {
				t.Fatalf("expected error but got nil")
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Validate metadata flag parsing
		if isMeta != expectedIsMeta {
			t.Fatalf("unexpected isMetadata: got %v, want %v", isMeta, expectedIsMeta)
		}

		// Validate data reconstruction
		if !bytes.Equal(resultData, expectedData) {
			t.Fatalf("unexpected data: got %q, want %q", resultData, expectedData)
		}
	}

	// Helper function: pack length header and payload into protocol-compliant binary stream
	buildPayload := func(header uint64, data []byte) []byte {
		var p []byte
		p = encoding.MarshalUint64(p, header)
		p = append(p, data...)
		return p
	}

	// Case 1: Normal data block (no metadata flag)
	// Highest bit = 0, data = "hello"
	f(buildPayload(5, []byte("hello")), 1024, []byte("hello"), false, false)

	// Case 2: Metadata block with flag set
	// Highest bit = 1, actual data length is still 5
	f(buildPayload(5|MaskMetadata, []byte("world")), 1024, []byte("world"), true, false)

	// Case 3: Zero-length normal data block (EOF marker)
	f(buildPayload(0, nil), 1024, []byte{}, false, false)

	// Case 4: Zero-length metadata block (EOF with metadata flag, e.g. isPartial scenario)
	f(buildPayload(0|MaskMetadata, nil), 1024, []byte{}, true, false)

	// Case 5: Data length exceeds maxDataSize, should return error
	f(buildPayload(100, []byte("data")), 50, nil, false, true)

	// Case 6: Metadata data length exceeds maxDataSize, should also return error
	f(buildPayload(100|MaskMetadata, []byte("data")), 50, nil, false, true)

	// Case 7: Incomplete length header (network truncation, less than 8 bytes)
	f([]byte{0x01, 0x02, 0x03, 0x04}, 1024, nil, false, true)

	// Case 8: Incomplete payload (declared length = 10, actual data = 5 bytes)
	f(buildPayload(10, []byte("short")), 1024, nil, false, true)
}
