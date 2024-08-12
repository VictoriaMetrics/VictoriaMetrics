package datasource

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"testing"
)

func BenchmarkMetrics(b *testing.B) {
	payload := []byte(`[{"metric":{"__name__":"vm_rows"},"value":[1583786142,"13763"]},{"metric":{"__name__":"vm_requests", "foo":"bar", "baz": "qux"},"value":[1583786140,"2000"]}]`)

	var pi promInstant
	if err := pi.Unmarshal(payload); err != nil {
		b.Fatalf(err.Error())
	}
	b.Run("Instant", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = pi.metrics()
		}
	})
}

func BenchmarkParsePrometheusResponse(b *testing.B) {
	req, _ := http.NewRequest("GET", "", nil)
	resp := &http.Response{StatusCode: http.StatusOK}
	data, err := os.ReadFile("testdata/instant_response.json")
	if err != nil {
		b.Fatalf("error while reading file: %s", err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))

	b.Run("Instant", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := parsePrometheusResponse(req, resp)
			if err != nil {
				b.Fatalf("unexpected parse err: %s", err)
			}
			resp.Body = io.NopCloser(bytes.NewReader(data))
		}
	})
}
