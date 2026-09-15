package httputil

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestSyncBodyTransportBufferReuse reuses a single buffer across requests to a
// server which doesn't read the request body. Run with -race.
func TestSyncBodyTransportBufferReuse(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer s.Close()

	client := &http.Client{
		Transport: NewSyncBodyTransport(http.DefaultTransport),
	}

	// The payload must exceed the socket buffers.
	payload := make([]byte, 8*1024*1024)

	var buf []byte
	for range 5 {
		buf = append(buf[:0], payload...)

		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, s.URL, bytes.NewReader(buf))
		if err != nil {
			t.Fatalf("cannot create request: %s", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("cannot send request: %s", err)
		}
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			t.Fatalf("cannot read response body: %s", err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("cannot close response body: %s", err)
		}
	}
}
