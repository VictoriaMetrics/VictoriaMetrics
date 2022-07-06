package promauth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VictoriaMetrics/fasthttp"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name         string
		opts         Options
		wantErr      bool
		expectHeader string
	}{
		{
			name: "OAuth2 config",
			opts: Options{
				OAuth2: &OAuth2Config{
					ClientID:     "some-id",
					ClientSecret: NewSecret("some-secret"),
					TokenURL:     "http://localhost:8511",
				},
			},
			expectHeader: "Bearer some-token",
		},
		{
			name: "OAuth2 config with file",
			opts: Options{
				OAuth2: &OAuth2Config{
					ClientID:         "some-id",
					ClientSecretFile: "testdata/test_secretfile.txt",
					TokenURL:         "http://localhost:8511",
				},
			},
			expectHeader: "Bearer some-token",
		},
		{
			name: "OAuth2 want err",
			opts: Options{
				OAuth2: &OAuth2Config{
					ClientID:         "some-id",
					ClientSecret:     NewSecret("some-secret"),
					ClientSecretFile: "testdata/test_secretfile.txt",
					TokenURL:         "http://localhost:8511",
				},
			},
			wantErr: true,
		},
		{
			name: "basic Auth config",
			opts: Options{
				BasicAuth: &BasicAuthConfig{
					Username: "user",
					Password: NewSecret("password"),
				},
			},
			expectHeader: "Basic dXNlcjpwYXNzd29yZA==",
		},
		{
			name: "basic Auth config with file",
			opts: Options{
				BasicAuth: &BasicAuthConfig{
					Username:     "user",
					PasswordFile: "testdata/test_secretfile.txt",
				},
			},
			expectHeader: "Basic dXNlcjpzZWNyZXQtY29udGVudA==",
		},
		{
			name: "want Authorization",
			opts: Options{
				Authorization: &Authorization{
					Type:        "Bearer",
					Credentials: NewSecret("Value"),
				},
			},
			expectHeader: "Bearer Value",
		},
		{
			name: "token file",
			opts: Options{
				BearerTokenFile: "testdata/test_secretfile.txt",
			},
			expectHeader: "Bearer secret-content",
		},
		{
			name: "token with tls",
			opts: Options{
				BearerToken: "some-token",
				TLSConfig: &TLSConfig{
					InsecureSkipVerify: true,
				},
			},
			expectHeader: "Bearer some-token",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.opts.OAuth2 != nil {
				r := http.NewServeMux()
				r.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"access_token":"some-token","token_type": "Bearer"}`))

				})
				mock := httptest.NewServer(r)
				tt.opts.OAuth2.TokenURL = mock.URL
			}
			got, err := tt.opts.NewConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil {
				req, err := http.NewRequest("GET", "http://foo", nil)
				if err != nil {
					t.Fatalf("unexpected error in http.NewRequest: %s", err)
				}
				got.SetHeaders(req, true)
				ah := req.Header.Get("Authorization")
				if ah != tt.expectHeader {
					t.Fatalf("unexpected auth header from net/http request; got %q; want %q", ah, tt.expectHeader)
				}
				var fhreq fasthttp.Request
				got.SetFasthttpHeaders(&fhreq, true)
				ahb := fhreq.Header.Peek("Authorization")
				if string(ahb) != tt.expectHeader {
					t.Fatalf("unexpected auth header from fasthttp request; got %q; want %q", ahb, tt.expectHeader)
				}
			}
		})
	}
}

func TestParseHeadersSuccess(t *testing.T) {
	f := func(headers []string) {
		t.Helper()
		headersParsed, err := parseHeaders(headers)
		if err != nil {
			t.Fatalf("unexpected error when parsing %s: %s", headers, err)
		}
		for i, h := range headersParsed {
			s := h.key + ": " + h.value
			if s != headers[i] {
				t.Fatalf("unexpected header parsed; got %q; want %q", s, headers[i])
			}
		}
	}
	f(nil)
	f([]string{"foo: bar"})
	f([]string{"Foo: bar", "A-b-c: d-e-f"})
}

func TestParseHeadersFailure(t *testing.T) {
	f := func(headers []string) {
		t.Helper()
		headersParsed, err := parseHeaders(headers)
		if err == nil {
			t.Fatalf("expecting non-nil error from parseHeaders(%s)", headers)
		}
		if headersParsed != nil {
			t.Fatalf("expecting nil result from parseHeaders(%s)", headers)
		}
	}
	f([]string{"foo"})
	f([]string{"foo bar baz"})
}

func TestConfigHeaders(t *testing.T) {
	f := func(headers []string, resultExpected string) {
		t.Helper()
		headersParsed, err := parseHeaders(headers)
		if err != nil {
			t.Fatalf("cannot parse headers: %s", err)
		}
		opts := Options{
			Headers: headers,
		}
		c, err := opts.NewConfig()
		if err != nil {
			t.Fatalf("cannot create config: %s", err)
		}
		req, err := http.NewRequest("GET", "http://foo", nil)
		if err != nil {
			t.Fatalf("unexpected error in http.NewRequest: %s", err)
		}
		result := c.HeadersNoAuthString()
		if result != resultExpected {
			t.Fatalf("unexpected result from HeadersNoAuthString; got\n%s\nwant\n%s", result, resultExpected)
		}
		c.SetHeaders(req, false)
		for _, h := range headersParsed {
			v := req.Header.Get(h.key)
			if v != h.value {
				t.Fatalf("unexpected value for net/http header %q; got %q; want %q", h.key, v, h.value)
			}
		}
		var fhreq fasthttp.Request
		c.SetFasthttpHeaders(&fhreq, false)
		for _, h := range headersParsed {
			v := fhreq.Header.Peek(h.key)
			if string(v) != h.value {
				t.Fatalf("unexpected value for fasthttp header %q; got %q; want %q", h.key, v, h.value)
			}
		}
	}
	f(nil, "")
	f([]string{"foo: bar"}, "foo: bar\r\n")
	f([]string{"Foo-Bar: Baz s:sdf", "A:b", "X-Forwarded-For: A-B:c"}, "Foo-Bar: Baz s:sdf\r\nA: b\r\nX-Forwarded-For: A-B:c\r\n")
}
