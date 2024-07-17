package main

import (
	"bytes"
	"io"
	"testing"
)

func TestReadTrackingBody_RetrySuccess(t *testing.T) {
	f := func(s string, maxBodySize int) {
		t.Helper()

		rtb := getReadTrackingBody(io.NopCloser(bytes.NewBufferString(s)), maxBodySize)
		defer putReadTrackingBody(rtb)

		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true before reading anything")
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

	f("", 0)
	f("", -1)
	f("", 100)
	f("foo", 100)
	f("foobar", 100)
	f(newTestString(1000), 1000)
}

func TestReadTrackingBody_RetrySuccessPartialRead(t *testing.T) {
	f := func(s string, maxBodySize int) {
		t.Helper()

		// Check the case with partial read
		rtb := getReadTrackingBody(io.NopCloser(bytes.NewBufferString(s)), maxBodySize)
		defer putReadTrackingBody(rtb)

		for i := 0; i < len(s); i++ {
			buf := make([]byte, i)
			n, err := io.ReadFull(rtb, buf)
			if err != nil {
				t.Fatalf("unexpected error when reading %d bytes: %s", i, err)
			}
			if n != i {
				t.Fatalf("unexpected number of bytes read; got %d; want %d", n, i)
			}
			if string(buf) != s[:i] {
				t.Fatalf("unexpected data read with the length %d\ngot\n%s\nwant\n%s", i, buf, s[:i])
			}
			if err := rtb.Close(); err != nil {
				t.Fatalf("unexpected error when closing reader after reading %d bytes", i)
			}
			if !rtb.canRetry() {
				t.Fatalf("canRetry() must return true after closing the reader after reading %d bytes", i)
			}
		}

		data, err := io.ReadAll(rtb)
		if err != nil {
			t.Fatalf("unexpected error when reading all the data: %s", err)
		}
		if string(data) != s {
			t.Fatalf("unexpected data read\ngot\n%s\nwant\n%s", data, s)
		}
		if err := rtb.Close(); err != nil {
			t.Fatalf("unexpected error when closing readTrackingBody: %s", err)
		}
		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true after closing the reader after reading all the input")
		}
	}

	f("", 0)
	f("", -1)
	f("", 100)
	f("foo", 100)
	f("foobar", 100)
	f(newTestString(1000), 1000)
}

func TestReadTrackingBody_RetryFailureTooBigBody(t *testing.T) {
	f := func(s string, maxBodySize int) {
		t.Helper()

		rtb := getReadTrackingBody(io.NopCloser(bytes.NewBufferString(s)), maxBodySize)
		defer putReadTrackingBody(rtb)

		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true before reading anything")
		}
		buf := make([]byte, 1)
		n, err := io.ReadFull(rtb, buf)
		if err != nil {
			t.Fatalf("unexpected error when reading a single byte: %s", err)
		}
		if n != 1 {
			t.Fatalf("unexpected number of bytes read; got %d; want 1", n)
		}
		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true after reading one byte")
		}
		data, err := io.ReadAll(rtb)
		if err != nil {
			t.Fatalf("unexpected error when reading all the data: %s", err)
		}
		dataRead := string(buf) + string(data)
		if dataRead != s {
			t.Fatalf("unexpected data read\ngot\n%s\nwant\n%s", dataRead, s)
		}
		if err := rtb.Close(); err != nil {
			t.Fatalf("unexpected error when closing readTrackingBody: %s", err)
		}
		if rtb.canRetry() {
			t.Fatalf("canRetry() must return false after closing the reader")
		}

		data, err = io.ReadAll(rtb)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if len(data) != 0 {
			t.Fatalf("unexpected non-empty data read: %q", data)
		}
	}

	const maxBodySize = 1000
	f(newTestString(maxBodySize+1), maxBodySize)
	f(newTestString(2*maxBodySize), maxBodySize)
}

func TestReadTrackingBody_RetryFailureZeroOrNegativeMaxBodySize(t *testing.T) {
	f := func(s string, maxBodySize int) {
		t.Helper()

		rtb := getReadTrackingBody(io.NopCloser(bytes.NewBufferString(s)), maxBodySize)
		defer putReadTrackingBody(rtb)

		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true before reading anything")
		}
		data, err := io.ReadAll(rtb)
		if err != nil {
			t.Fatalf("unexpected error when reading all the data: %s", err)
		}
		if string(data) != s {
			t.Fatalf("unexpected data read\ngot\n%s\nwant\n%s", data, s)
		}
		if err := rtb.Close(); err != nil {
			t.Fatalf("unexpected error when closing readTrackingBody: %s", err)
		}

		if rtb.canRetry() {
			t.Fatalf("canRetry() must return false after closing the reader")
		}
		data, err = io.ReadAll(rtb)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if len(data) != 0 {
			t.Fatalf("unexpected non-empty data read: %q", data)
		}
	}

	f("foobar", 0)
	f(newTestString(1000), 0)

	f("foobar", -1)
	f(newTestString(1000), -1)
}

func newTestString(sLen int) string {
	data := make([]byte, sLen)
	for i := range data {
		data[i] = byte(i)
	}
	return string(data)
}
