package azure

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	azlog "github.com/Azure/azure-sdk-for-go/sdk/azcore/log"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

func sdkSecurityConfig(t *testing.T, server *httptest.Server) *SDConfig {
	t.Helper()
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	return &SDConfig{
		Environment: "AzureStackCloud", AuthenticationMethod: "WorkloadIdentity",
		HTTPClientConfig: promauth.HTTPClientConfig{TLSConfig: &promauth.TLSConfig{CAFile: caFile}},
	}
}

func sdkSecurityClient(t *testing.T, cfg *SDConfig) *sdkHTTPClient {
	t.Helper()
	ac, err := cfg.HTTPClientConfig.NewConfig("")
	if err != nil {
		t.Fatal(err)
	}
	pac, err := cfg.ProxyClientConfig.NewConfig("")
	if err != nil {
		t.Fatal(err)
	}
	return newSDKHTTPClient(cfg, ac, pac)
}

// These tests are deliberately not parallel: the SDK listener is process-wide.
// Use the real SDK and trusted TLS transport, not a stub TokenCredential.
func TestSDKHTTPErrorLogging(t *testing.T) {
	for _, scenario := range []string{"success", "json-error", "header-error", "body-error", "retry"} {
		t.Run(scenario, func(t *testing.T) {
			workloadEnv(t)
			var mu sync.Mutex
			var messages []string
			azlog.SetListener(func(event azlog.Event, message string) {
				mu.Lock()
				messages = append(messages, string(event)+": "+message)
				mu.Unlock()
			})
			defer azlog.SetListener(nil)
			var authority string
			var tokenCalls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
					fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q}`, authority+"/env-tenant/v2.0", authority+"/env-tenant/oauth2/v2.0/authorize", authority+"/env-tenant/oauth2/v2.0/token")
					return
				}
				if !strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token") {
					t.Error("unexpected endpoint")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				tokenCalls.Add(1)
				if err := r.ParseForm(); err != nil {
					t.Error(err)
					return
				}
				assertion := r.Form.Get("client_assertion")
				if assertion != "synthetic-assertion" {
					t.Error("missing synthetic assertion")
				}
				if scenario == "retry" && tokenCalls.Load() > 1 {
					fmt.Fprint(w, `{"access_token":"synthetic-access","expires_in":3600,"token_type":"Bearer"}`)
					return
				}
				switch scenario {
				case "success":
					fmt.Fprint(w, `{"access_token":"synthetic-access","expires_in":3600,"token_type":"Bearer"}`)
				case "json-error":
					w.WriteHeader(http.StatusUnauthorized)
					fmt.Fprintf(w, `{"error":"invalid_client","error_description":%q}`, assertion)
				default:
					conn, b, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					defer conn.Close()
					if scenario == "header-error" || scenario == "retry" {
						fmt.Fprintf(b, "HTTP/1.1 200 OK\r\n%s\r\n\r\n", assertion)
					} else {
						// Valid headers, then a malformed chunked trailer: the error
						// containing the assertion occurs in Body.Read, after Do.
						fmt.Fprintf(b, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nTransfer-Encoding: chunked\r\nTrailer: X-Test\r\n\r\n2\r\n{}\r\n0\r\n%s\r\n\r\n", assertion)
					}
					if err := b.Flush(); err != nil {
						t.Error(err)
					}
				}
			}))
			defer server.Close()
			authority = server.URL
			cfg := sdkSecurityConfig(t, server)
			client := sdkSecurityClient(t, cfg)
			retries := int32(-1)
			if scenario == "retry" || scenario == "body-error" {
				// Transport failures remain retryable, but the SDK must still
				// reject retries after a POST response body download failure.
				retries = 1
			}
			ctx := policy.WithRetryOptions(context.Background(), policy.RetryOptions{MaxRetries: retries, RetryDelay: time.Millisecond})
			refresh, err := newSDKRefreshTokenFunc(ctx, cfg, client.ac, client.proxyAC, &cloudEnvironmentEndpoints{ActiveDirectoryEndpoint: authority, ResourceManagerEndpoint: "https://resource.invalid"})
			if err != nil {
				t.Fatal(err)
			}
			token, _, err := refresh()
			wantCalls := int32(1)
			if scenario == "retry" {
				wantCalls = 2
			}
			if tokenCalls.Load() != wantCalls {
				t.Fatalf("unexpected token calls: %d", tokenCalls.Load())
			}
			if scenario == "success" || scenario == "retry" {
				if err != nil || token != "synthetic-access" {
					t.Fatal("failed synthetic success")
				}
			} else if err == nil || token != "" {
				t.Fatal("failed synthetic error control")
			}
			if err != nil && strings.Contains(fmt.Sprintf("%+v", err), "synthetic-") {
				t.Fatal("returned error disclosed synthetic credential")
			}
			mu.Lock()
			joined := strings.Join(messages, "\n")
			mu.Unlock()
			if !strings.Contains(joined, "Request:") || !strings.Contains(joined, "Response:") || !strings.Contains(joined, "Retry:") {
				t.Fatal("SDK request, response and retry logging must be active")
			}
			if strings.Contains(joined, "synthetic-assertion") || strings.Contains(joined, "synthetic-access") {
				t.Fatal("SDK logs disclosed synthetic credential")
			}
			t.Logf("SDK logging active: %d messages; token calls=%d; no credential markers", len(messages), tokenCalls.Load())
		})
	}
}

type sdkSecurityRoundTripper func(*http.Request) (*http.Response, error)

func (f sdkSecurityRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type sdkSecurityBody struct {
	err    error
	closed bool
}

func (b *sdkSecurityBody) Read(p []byte) (int, error) {
	return copy(p, "{}"), b.err
}

func (b *sdkSecurityBody) Close() error {
	b.closed = true
	return b.err
}

type sdkSecurityTimeoutError struct{}

func (sdkSecurityTimeoutError) Error() string   { return "synthetic-transport-secret" }
func (sdkSecurityTimeoutError) Timeout() bool   { return true }
func (sdkSecurityTimeoutError) Temporary() bool { return true }

func TestSDKHTTPErrorBoundary(t *testing.T) {
	f := func(cause error, want error, timeout bool) {
		t.Helper()
		client := sdkSecurityClient(t, &SDConfig{})
		body := &sdkSecurityBody{err: cause}
		client.client.Transport = sdkSecurityRoundTripper(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: body, Request: req}, nil
		})
		req, err := http.NewRequest(http.MethodGet, "https://offline.invalid", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 2)
		n, readErr := resp.Body.Read(buf)
		if n != 2 || string(buf) != "{}" {
			t.Fatal("body bytes were not preserved")
		}
		closeErr := resp.Body.Close()
		if !body.closed {
			t.Fatal("body was not closed")
		}
		client.client.Transport = sdkSecurityRoundTripper(func(*http.Request) (*http.Response, error) { return nil, cause })
		_, transportErr := client.Do(req)
		for _, got := range []error{readErr, closeErr, transportErr} {
			if got == nil || strings.Contains(fmt.Sprintf("%v %+v %#v", got, got, got), "synthetic-") || errors.Is(got, cause) && want == nil {
				t.Fatal("raw HTTP error escaped the SDK boundary")
			}
			if want != nil && !errors.Is(got, want) {
				t.Fatalf("lost error identity: want %v, got %v", want, got)
			}
			if timeout {
				var ne net.Error
				if !errors.As(got, &ne) || !ne.Timeout() {
					t.Fatal("timeout classification lost")
				}
			}
		}
	}
	f(errors.New("synthetic-body-secret"), nil, false)
	f(fmt.Errorf("synthetic-secret: %w", context.Canceled), context.Canceled, false)
	f(fmt.Errorf("synthetic-secret: %w", context.DeadlineExceeded), context.DeadlineExceeded, true)
	f(sdkSecurityTimeoutError{}, nil, true)
	f(io.ErrUnexpectedEOF, io.ErrUnexpectedEOF, false)
	f(io.EOF, io.EOF, false)
}

func TestSDKHTTPHeaderErrors(t *testing.T) {
	for _, useProxy := range []bool{false, true} {
		cfg := &SDConfig{}
		missing := filepath.Join(t.TempDir(), "synthetic-secret-missing")
		if useProxy {
			cfg.ProxyURL = proxy.MustNewURL("http://offline.invalid")
			cfg.ProxyClientConfig.BearerTokenFile = missing
		} else {
			cfg.HTTPClientConfig.BearerTokenFile = missing
		}
		client := sdkSecurityClient(t, cfg)
		client.client.Transport = sdkSecurityRoundTripper(func(*http.Request) (*http.Response, error) {
			t.Error("request sent despite header error")
			return nil, errors.New("unexpected request")
		})
		req, err := http.NewRequest(http.MethodGet, "https://offline.invalid", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = client.Do(req)
		if err == nil || strings.Contains(fmt.Sprintf("%v %+v %#v", err, err, err), "synthetic-secret") {
			t.Fatal("header error not sanitized")
		}
	}
}

// Cancellation must survive the real SDK's error wrapping, not only a fake credential.
func TestSDKHTTPContextErrors(t *testing.T) {
	for _, deadline := range []bool{false, true} {
		t.Run(fmt.Sprintf("deadline=%t", deadline), func(t *testing.T) {
			workloadEnv(t)
			started := make(chan struct{})
			release := make(chan struct{})
			var authority string
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
					fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q}`, authority+"/env-tenant/v2.0", authority+"/env-tenant/oauth2/v2.0/authorize", authority+"/env-tenant/oauth2/v2.0/token")
					return
				}
				if !strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token") {
					t.Error("unexpected endpoint")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				close(started)
				select {
				case <-r.Context().Done():
				case <-release:
				}
			}))
			defer server.Close()
			defer close(release)
			authority = server.URL
			cfg := sdkSecurityConfig(t, server)
			client := sdkSecurityClient(t, cfg)
			discovery, err := discoveryutil.NewClient(authority, client.ac, nil, client.proxyAC, &cfg.HTTPClientConfig)
			if err != nil {
				t.Fatal(err)
			}
			defer discovery.Stop()
			ctx := discovery.Context()
			want := context.Canceled
			if deadline {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
				defer cancel()
				want = context.DeadlineExceeded
			}
			refresh, err := newSDKRefreshTokenFunc(ctx, cfg, client.ac, client.proxyAC, &cloudEnvironmentEndpoints{ActiveDirectoryEndpoint: authority, ResourceManagerEndpoint: "https://resource.invalid"})
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { _, _, err := refresh(); done <- err }()
			select {
			case <-started:
			case <-time.After(10 * time.Second):
				t.Fatal("token request did not start")
			}
			if !deadline {
				discovery.Stop()
			}
			select {
			case err := <-done:
				if !errors.Is(err, want) {
					t.Fatalf("context error lost: got %v, want %v", err, want)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("token request did not stop")
			}
		})
	}
}

func TestSDKHTTPRedirectSecurity(t *testing.T) {
	for _, status := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			for _, scenario := range []string{"downgrade-default", "downgrade-enabled", "https", "no-follow", "limit"} {
				t.Run(scenario, func(t *testing.T) {
					workloadEnv(t)
					var plainCalls, tokenCalls, secureCalls atomic.Int32
					plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						plainCalls.Add(1)
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, `{"access_token":"synthetic-access","expires_in":3600,"token_type":"Bearer"}`)
					}))
					defer plain.Close()
					var authority string
					server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
							fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q}`, authority+"/env-tenant/v2.0", authority+"/env-tenant/oauth2/v2.0/authorize", authority+"/env-tenant/oauth2/v2.0/token")
							return
						}
						if err := r.ParseForm(); err != nil {
							t.Error(err)
							return
						}
						if r.Method != http.MethodPost || r.Form.Get("client_assertion") != "synthetic-assertion" || r.Header.Get("X-Test-Secret") != "synthetic-header" {
							t.Error("missing credential-bearing POST or custom header")
						}
						if r.URL.Path == "/receive" {
							secureCalls.Add(1)
							fmt.Fprint(w, `{"access_token":"synthetic-access","expires_in":3600,"token_type":"Bearer"}`)
							return
						}
						if !strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token") {
							t.Error("unexpected endpoint")
							w.WriteHeader(http.StatusBadRequest)
							return
						}
						tokenCalls.Add(1)
						location := authority + "/receive"
						if strings.HasPrefix(scenario, "downgrade-") {
							location = plain.URL + "/receive"
						} else if scenario == "limit" {
							location = authority + r.URL.Path
						}
						w.Header().Set("Location", location)
						w.WriteHeader(status)
					}))
					defer server.Close()
					authority = server.URL
					cfg := sdkSecurityConfig(t, server)
					cfg.HTTPClientConfig.Headers = []string{"X-Test-Secret: synthetic-header"}
					if scenario == "downgrade-enabled" || scenario == "no-follow" {
						follow := scenario != "no-follow"
						cfg.HTTPClientConfig.FollowRedirects = &follow
					}
					client := sdkSecurityClient(t, cfg)
					ctx := policy.WithRetryOptions(context.Background(), policy.RetryOptions{MaxRetries: -1})
					refresh, err := newSDKRefreshTokenFunc(ctx, cfg, client.ac, client.proxyAC, &cloudEnvironmentEndpoints{ActiveDirectoryEndpoint: authority, ResourceManagerEndpoint: "https://resource.invalid"})
					if err != nil {
						t.Fatal(err)
					}
					token, _, err := refresh()
					if plainCalls.Load() != 0 {
						t.Fatalf("HTTPS redirect reached plain HTTP receiver: %d requests", plainCalls.Load())
					}
					wantCalls := int32(1)
					if scenario == "limit" {
						wantCalls = 10
					}
					if tokenCalls.Load() != wantCalls {
						t.Fatalf("wrong redirect limit: got %d token calls, want %d", tokenCalls.Load(), wantCalls)
					}
					if scenario == "https" {
						if err != nil || token != "synthetic-access" || secureCalls.Load() != 1 {
							t.Fatal("safe HTTPS redirect failed")
						}
					} else if err == nil || token != "" || secureCalls.Load() != 0 {
						t.Fatal("forbidden redirect accepted")
					}
					t.Logf("plain requests=%d; HTTPS receiver requests=%d; token endpoint requests=%d", plainCalls.Load(), secureCalls.Load(), tokenCalls.Load())
				})
			}
		})
	}
}
