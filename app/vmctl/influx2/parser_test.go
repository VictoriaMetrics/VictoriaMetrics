package influx2

import (
	"encoding/json"
	"testing"
)

func TestToFloat64_Success(t *testing.T) {
	f := func(in any, want float64) {
		t.Helper()
		got, err := toFloat64(in)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if got != want {
			t.Fatalf("got %v; want %v", got, want)
		}
	}

	f("123.4", 123.4)
	f(float64(123.4), 123.4)
	f(float32(12), 12)
	f(int64(99), 99)
	f(int(5), 5)
	f(true, 1)
	f(false, 0)
	f(json.Number("123456.789"), 123456.789)
}

func TestToFloat64_Failure(t *testing.T) {
	// "text" can't be parsed as a float — should return an error.
	_, err := toFloat64("text")
	if err == nil {
		t.Fatal("expected error for non-numeric string, got nil")
	}
}
