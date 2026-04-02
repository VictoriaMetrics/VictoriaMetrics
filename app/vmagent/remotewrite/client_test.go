package remotewrite

import (
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

func TestParseRetryAfterHeader(t *testing.T) {
	f := func(retryAfterString string, expectResult time.Duration) {
		t.Helper()

		result := parseRetryAfterHeader(retryAfterString)
		// expect `expectResult == result` when retryAfterString is in seconds or invalid
		// expect the difference between result and expectResult to be lower than 10%
		if !(expectResult == result || math.Abs(float64(expectResult-result))/float64(expectResult) < 0.10) {
			t.Fatalf(
				"incorrect retry after duration, want (ms): %d, got (ms): %d",
				expectResult.Milliseconds(), result.Milliseconds(),
			)
		}
	}

	// retry after header in seconds
	f("10", 10*time.Second)
	// retry after header in date time
	f(time.Now().Add(30*time.Second).UTC().Format(http.TimeFormat), 30*time.Second)
	// retry after header invalid
	f("invalid-retry-after", 0)
	// retry after header not in GMT
	f(time.Now().Add(10*time.Second).Format("Mon, 02 Jan 2006 15:04:05 FAKETZ"), 0)
}

func TestRepackBlockFromZstdToSnappy(t *testing.T) {
	expectedPlainBlock := []byte(`foobar`)

	zstdBlock := encoding.CompressZSTDLevel(nil, expectedPlainBlock, 1)
	snappyBlock, err := repackBlockFromZstdToSnappy(zstdBlock)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	actualPlainBlock, err := snappy.Decode(nil, snappyBlock)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if string(actualPlainBlock) != string(expectedPlainBlock) {
		t.Fatalf("unexpected plain block; got %q; want %q", actualPlainBlock, expectedPlainBlock)
	}
}

func TestRepackBlockFromZstdToSnappyInvalidBlock(t *testing.T) {
	snappyBlock, err := repackBlockFromZstdToSnappy([]byte("invalid zstd block"))

	if err == nil {
		t.Fatalf("expected error for invalid zstd block; got nil")
	}
	if len(snappyBlock) != 0 {
		t.Fatalf("expected empty snappy block; got %d bytes", len(snappyBlock))
	}
}
