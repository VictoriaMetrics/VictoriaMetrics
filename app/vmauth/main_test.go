package main

import (
	"bytes"
	"io"
	"testing"
)

func TestReadTrackingBodyRetrySuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		rtb := &readTrackingBody{
			r: io.NopCloser(bytes.NewBufferString(s)),
		}
		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true")
		}
		for i := 0; i < 5; i++ {
			data, err := io.ReadAll(rtb)
			if err != nil {
				t.Fatalf("unexpected error when reading all the data at iteration %d: %s", i, err)
			}
			if string(data) != s {
				t.Fatalf("unexpected data read at iteration %d\ngot\n%s\nwant\n%s", i, data, s)
			}
			if err := rtb.Close(); err != nil {
				t.Fatalf("unexpected error when closing readTrackingBody at iteration %d: %s", i, err)
			}
			if !rtb.canRetry() {
				t.Fatalf("canRetry() must return true at iteration %d", i)
			}
		}
	}

	f("")
	f("foo")
	f("foobar")
	f(newTestString(maxRequestBodySizeToRetry.IntN()))
}

func TestReadTrackingBodyRetryFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		rtb := &readTrackingBody{
			r: io.NopCloser(bytes.NewBufferString(s)),
		}
		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true")
		}
		buf := make([]byte, 1)
		n, err := rtb.Read(buf)
		if err != nil {
			t.Fatalf("unexpected error when reading a single byte: %s", err)
		}
		if n != 1 {
			t.Fatalf("unexpected number of bytes read; got %d; want 1", n)
		}
		if rtb.canRetry() {
			t.Fatalf("canRetry() must return false")
		}
		data, err := io.ReadAll(rtb)
		if err != nil {
			t.Fatalf("unexpected error when reading all the data: %s", err)
		}
		if string(buf)+string(data) != s {
			t.Fatalf("unexpected data read\ngot\n%s\nwant\n%s", string(buf)+string(data), s)
		}
		if err := rtb.Close(); err != nil {
			t.Fatalf("unexpected error when closing readTrackingBody: %s", err)
		}
		if rtb.canRetry() {
			t.Fatalf("canRetry() must return false")
		}

		data, err = io.ReadAll(rtb)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if len(data) != 0 {
			t.Fatalf("unexpected non-empty data read: %q", data)
		}
	}

	f(newTestString(maxRequestBodySizeToRetry.IntN() + 1))
	f(newTestString(2 * maxRequestBodySizeToRetry.IntN()))
}

func newTestString(sLen int) string {
	return string(make([]byte, sLen))
}
