package httpserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestBasicAuthMetrics(t *testing.T) {
	origUsername := *httpAuthUsername
	origPasswd := *httpAuthPassword
	testsForConfiguredAuth := []struct {
		username     string
		passwd       string
		expectedCode int
	}{
		{
			username:     "test",
			passwd:       "pass",
			expectedCode: 200,
		},
		{
			username:     "test",
			passwd:       "wrong",
			expectedCode: 401,
		},
		{
			username:     "wrongUser",
			passwd:       "pass",
			expectedCode: 401,
		},
		{
			username:     "wrongUser",
			passwd:       "wrongpass",
			expectedCode: 401,
		},
	}

	testsWhenAuthIsNotConfigured := []struct {
		username     string
		passwd       string
		expectedCode int
	}{
		{
			username:     "test",
			passwd:       "pass",
			expectedCode: 200,
		},
		{
			username:     "test",
			passwd:       "wrong",
			expectedCode: 200,
		},
		{
			username:     "wrongUser",
			passwd:       "pass",
			expectedCode: 200,
		},
		{
			username:     "wrongUser",
			passwd:       "wrongpass",
			expectedCode: 200,
		},
	}
	*httpAuthUsername = "test"
	*httpAuthPassword = "pass"
	for _, tt := range testsForConfiguredAuth {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.SetBasicAuth(tt.username, tt.passwd)

		w := httptest.NewRecorder()
		CheckBasicAuth(w, req)

		res := w.Result()
		defer res.Body.Close()
		if tt.expectedCode != res.StatusCode {
			t.Fatalf("Unexpected status code: %d, Expected code is: %d\n", res.StatusCode, tt.expectedCode)
		}
	}

	*httpAuthUsername = ""
	*httpAuthPassword = ""
	for _, tt := range testsWhenAuthIsNotConfigured {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.SetBasicAuth(tt.username, tt.passwd)

		w := httptest.NewRecorder()
		CheckBasicAuth(w, req)

		res := w.Result()
		defer res.Body.Close()
		if tt.expectedCode != res.StatusCode {
			t.Fatalf("Unexpected status code: %d, Expected code is: %d\n", res.StatusCode, tt.expectedCode)
		}
	}

	*httpAuthPassword = origPasswd
	*httpAuthUsername = origUsername

}

func TestAuthKeyMetrics(t *testing.T) {
	origUsername := *httpAuthUsername
	origPasswd := *httpAuthPassword
	testsForConfiguredAuthKey := []struct {
		authKey      string
		expectedCode int
	}{
		{
			authKey:      "test",
			expectedCode: 200,
		},
		{
			authKey:      "wrongUser",
			expectedCode: 401,
		},
	}

	testsWhenAuthKeyIsNotConfigured := []struct {
		username     string
		passwd       string
		expectedCode int
	}{
		{
			username:     "test",
			passwd:       "pass",
			expectedCode: 200,
		},
		{
			username:     "test",
			passwd:       "wrong",
			expectedCode: 401,
		},
		{
			username:     "wrongUser",
			passwd:       "pass",
			expectedCode: 401,
		},
		{
			username:     "wrongUser",
			passwd:       "wrongpass",
			expectedCode: 401,
		},
	}
	authKey := "test"
	for _, tt := range testsForConfiguredAuthKey {
		req := httptest.NewRequest(http.MethodPost, "/metrics", strings.NewReader("authKey="+tt.authKey))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded;param=value")
		w := httptest.NewRecorder()

		CheckAuthFlag(w, req, authKey, "metricsAuthkey")

		res := w.Result()
		defer res.Body.Close()
		if tt.expectedCode != res.StatusCode {
			t.Fatalf("Unexpected status code: %d, Expected code is: %d\n", res.StatusCode, tt.expectedCode)
		}
	}

	*httpAuthUsername = "test"
	*httpAuthPassword = "pass"
	for _, tt := range testsWhenAuthKeyIsNotConfigured {
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		req.SetBasicAuth(tt.username, tt.passwd)

		w := httptest.NewRecorder()
		CheckAuthFlag(w, req, "", "metricsAuthKey")

		res := w.Result()
		defer res.Body.Close()
		if tt.expectedCode != res.StatusCode {
			t.Fatalf("Unexpected status code: %d, Expected code is: %d\n", res.StatusCode, tt.expectedCode)
		}
	}

	*httpAuthPassword = origPasswd
	*httpAuthUsername = origUsername

}
