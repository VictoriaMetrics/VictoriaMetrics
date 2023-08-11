package httpserver

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetQuotedRemoteAddr(t *testing.T) {
	f := func(remoteAddr, xForwardedFor, expectedAddr string) {
		t.Helper()

		req := &http.Request{
			RemoteAddr: remoteAddr,
		}
		if xForwardedFor != "" {
			req.Header = map[string][]string{
				"X-Forwarded-For": {xForwardedFor},
			}
		}
		addr := GetQuotedRemoteAddr(req)
		if addr != expectedAddr {
			t.Fatalf("unexpected remote addr;\ngot\n%s\nwant\n%s", addr, expectedAddr)
		}

		// Verify that the addr can be unmarshaled as JSON string
		var s string
		if err := json.Unmarshal([]byte(addr), &s); err != nil {
			t.Fatalf("cannot unmarshal addr: %s", err)
		}
	}

	f("1.2.3.4", "", `"1.2.3.4"`)
	f("1.2.3.4", "foo.bar", `"1.2.3.4, X-Forwarded-For: foo.bar"`)
	f("1.2\n\"3.4", "foo\nb\"ar", `"1.2\n\"3.4, X-Forwarded-For: foo\nb\"ar"`)
}
