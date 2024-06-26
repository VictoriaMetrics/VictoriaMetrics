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

// request body not over maxRequestBodySizeToRetry
// 1. When writing data downstream, buf only caches part of the data because the downstream connection is disconnected.
// 2. retry request: because buf caches some data, first read buf and then read stream when retrying
// 3. retry request: the data has been read to buf in the second step. if the request fails, retry to read all buf later.
func TestRetryReadSuccessAfterPartialRead(t *testing.T) {
	f := func(s string) {
		rtb := &readTrackingBody{
			r:   io.NopCloser(bytes.NewBufferString(s)),
			buf: make([]byte, 0, len(s)),
		}

		var data []byte
		var err error
		halfSize := len(s) / 2
		if halfSize == 0 {
			halfSize = 100
		}
		buf := make([]byte, halfSize)
		var n int

		// read part of the data
		n, err = rtb.Read(buf[:])
		data = append(data, buf[:n]...)
		if err != nil && err != io.EOF {
			t.Fatalf("unexpected error: %s", err)
		}

		// request failed when output stream is closed (eg: server connection reset)
		// would close the reader
		if err := rtb.Close(); err != nil {
			t.Fatalf("unexpected error when closing readTrackingBody: %s", err)
		}
		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true")
		}

		// retry read (read buf + remaining data)
		data = data[:0]
		err = nil
		for err == nil {
			n, err = rtb.Read(buf[:])
			data = append(data, buf[:n]...)
		}
		if err != io.EOF {
			t.Fatalf("unexpected error: %s", err)
		}
		if string(data) != s {
			t.Fatalf("unexpected data read; got\n%s\nwant\n%s", data, s)
		}
		// cannotRetry return false
		// because the request data is not over maxRequestBodySizeToRetry limit
		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true")
		}
	}

	f("")
	f("foo")
	f("foobar")
	f(newTestString(maxRequestBodySizeToRetry.IntN()))
}

// request body over maxRequestBodySizeToRetry
// 1. When writing data downstream, buf only caches part of the data because the downstream connection is disconnected.
// 2. retry request: because buf caches some data, first read buf and then read stream when retrying
// 3. retry request: the data has been read to buf in the second step. if the request fails, retry to read all buf later.
func TestRetryReadSuccessAfterPartialReadAndCannotRetryAgain(t *testing.T) {
	f := func(s string) {
		rtb := &readTrackingBody{
			r:   io.NopCloser(bytes.NewBufferString(s)),
			buf: make([]byte, 0, len(s)),
		}

		var data []byte
		var err error
		halfSize := len(s) / 2
		if halfSize == 0 {
			halfSize = 100
		}
		buf := make([]byte, halfSize)
		var n int

		// read part of the data
		n, err = rtb.Read(buf[:])
		data = append(data, buf[:n]...)
		if err != nil && err != io.EOF {
			t.Fatalf("unexpected error: %s", err)
		}

		// request failed when output stream is closed (eg: server connection reset)
		if err := rtb.Close(); err != nil {
			t.Fatalf("unexpected error when closing readTrackingBody: %s", err)
		}
		if !rtb.canRetry() {
			t.Fatalf("canRetry() must return true")
		}

		// retry read (read buf + remaining data)
		data = data[:0]
		err = nil
		for err == nil {
			n, err = rtb.Read(buf[:])
			data = append(data, buf[:n]...)
		}
		if err != io.EOF {
			t.Fatalf("unexpected error: %s", err)
		}
		if string(data) != s {
			t.Fatalf("unexpected data read; got\n%s\nwant\n%s", data, s)
		}

		// cannotRetry returns true
		// because the request data is over maxRequestBodySizeToRetry limit
		if rtb.canRetry() {
			t.Fatalf("canRetry() must return false")
		}
	}

	f(newTestString(maxRequestBodySizeToRetry.IntN() + 1))
	f(newTestString(2 * maxRequestBodySizeToRetry.IntN()))
}

func newTestString(sLen int) string {
	return string(make([]byte, sLen))
}
