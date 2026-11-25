package fscore

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadFileOrHTTP(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "groups: []\n")
		}))
		defer srv.Close()

		got, err := ReadFileOrHTTP(srv.URL)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "groups: []\n" {
			t.Fatalf("got %q", got)
		}
	})

	t.Run("too big", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.Copy(w, io.LimitReader(repeatReader('a'), int64(maxHTTPBodySize)+1))
		}))
		defer srv.Close()

		_, err := ReadFileOrHTTP(srv.URL)
		if err == nil {
			t.Fatal("expected a size error")
		}
		if !strings.Contains(err.Error(), "too big response") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("non-200 truncated", func(t *testing.T) {
		const marker = "TAILMARKER"
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.Copy(w, io.LimitReader(repeatReader('b'), int64(maxHTTPBodySize)+1-int64(len(marker))))
			_, _ = io.WriteString(w, marker)
		}))
		defer srv.Close()

		_, err := ReadFileOrHTTP(srv.URL)
		if err == nil {
			t.Fatal("expected a status error")
		}
		msg := err.Error()
		if !strings.Contains(msg, "unexpected status code") || !strings.Contains(msg, "500") {
			t.Fatalf("got %v", err)
		}
		if strings.Contains(msg, "too big response") || strings.Contains(msg, marker) {
			t.Fatalf("status error should keep a 4KB snippet and the status, got %v", err)
		}
	})
}

type repeatReader byte

func (r repeatReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(r)
	}
	return len(p), nil
}
