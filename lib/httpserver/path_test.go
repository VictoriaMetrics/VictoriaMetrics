package httpserver

import (
	"net/http"
	"strings"
	"testing"
)

func TestParsePathAndHeadersSuccess(t *testing.T) {
	f := func(path, headers, prefix, authToken, suffix string) {
		t.Helper()
		header := make(http.Header)
		hs := strings.Split(headers, ";")
		for _, h := range hs {
			if h == "" {
				continue
			}
			parts := strings.Split(h, ":")
			header.Set(parts[0], parts[1])
		}
		p, err := ParsePathAndHeaders(path, header)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if p.Prefix != prefix {
			t.Fatalf("unexpected Prefix; got %q; want %q", p.Prefix, prefix)
		}
		if p.AuthToken != authToken {
			t.Fatalf("unexpected AuthToken; got %q; want %q", p.AuthToken, authToken)
		}
		if p.Suffix != suffix {
			t.Fatalf("unexpected Suffix; got %q; want %q", p.Suffix, suffix)
		}
	}

	// tenant is omitted in the Path, so we try reading it from headers
	f("/select/prometheus/api/v1/query", "AccountID:1;ProjectID:1", "select", "1:1", "prometheus/api/v1/query")
	f("/select/prometheus/api/v1/query_range", "AccountID:1", "select", "1:0", "prometheus/api/v1/query_range")
	f("/select/prometheus/api/v1/query_range", "ProjectID:1", "select", "0:1", "prometheus/api/v1/query_range")
	f("/insert/prometheus", "", "insert", "0:0", "prometheus")
	f("/insert/prometheus", "AccountID:1;ProjectID:1", "insert", "1:1", "prometheus")
	f("/insert/prometheus", "AccountID:multitenant", "insert", "multitenant", "prometheus")
	f("/delete/prometheus/api/v1/admin/tsdb/delete_series", "AccountID:1;ProjectID:1", "delete", "1:1", "prometheus/api/v1/admin/tsdb/delete_series")
	f("/insert//prometheus/api/v1/import/prometheus", "AccountID:2", "insert", "2:0", "prometheus/api/v1/import/prometheus")

	// If headers are empty, we assume 0:0 as default.
	f("/insert/prometheus", "", "insert", "0:0", "prometheus")
	f("/select/prometheus/api/v1/query", "", "select", "0:0", "prometheus/api/v1/query")

	// tenant is present in the Path
	f("/insert/123/prometheus/api/v1/write", "", "insert", "123", "prometheus/api/v1/write")
	f("/select/1:15/prometheus/api/v1/query", "", "select", "1:15", "prometheus/api/v1/query")
	f("/insert/multitenant/prometheus/api/v1/write", "", "insert", "multitenant", "prometheus/api/v1/write")
	f("/insert/0/prometheus/api/v1/write", "", "insert", "0", "prometheus/api/v1/write")
	f("/insert/0:0/prometheus/api/v1/write", "", "insert", "0:0", "prometheus/api/v1/write")
	f("/delete/123/prometheus/api/v1/admin/tsdb/delete_series", "", "delete", "123", "prometheus/api/v1/admin/tsdb/delete_series")

	// tenant in the Path takes priority over headers
	f("/insert/123/prometheus/api/v1/write", "AccountID:1;ProjectID:1", "insert", "123", "prometheus/api/v1/write")
	f("/insert/multitenant/prometheus/api/v1/write", "AccountID:1;ProjectID:1", "insert", "multitenant", "prometheus/api/v1/write")
	f("/insert/123:1/prometheus/api/v1/write", "AccountID:multitenant", "insert", "123:1", "prometheus/api/v1/write")

	// Double slashes in Path
	f("//insert//123//prometheus//api/v1/write", "", "insert", "123", "prometheus/api/v1/write")
	f("//insert//prometheus//api/v1/write", "AccountID:1;ProjectID:1", "insert", "1:1", "prometheus/api/v1/write")

}

func TestParsePathAndHeadersFailure(t *testing.T) {
	f := func(path string) {
		t.Helper()
		p, err := ParsePathAndHeaders(path, nil)
		if err == nil {
			t.Fatalf("expecting non-nil error; got path %+v", p)
		}
	}

	// No prefix or suffix
	f("/")
	// Only prefix, no suffix or tenant
	f("/insert")
	f("/select/")
	f("/select//")
}
