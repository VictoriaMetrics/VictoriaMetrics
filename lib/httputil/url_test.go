package httputil

import (
	"testing"
)

func TestCheckURL_Success(t *testing.T) {
	f := func(urlStr string) {
		t.Helper()

		if err := CheckURL(urlStr); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}

	f("http://victoriametrics.com")
	f("https://victoriametrics.com")
}

func TestCheckURL_Failure(t *testing.T) {
	f := func(urlStr string) {
		t.Helper()

		if err := CheckURL(urlStr); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// empty url
	f("")

	// no schema
	f("127.0.0.1:8880")
}
