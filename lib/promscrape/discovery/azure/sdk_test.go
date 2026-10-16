package azure

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// Do not inherit host identities. DefaultAzureCredential tests select only a
// synthetic workload/environment credential, never developer tools or IMDS.
func isolateAzureEnv(t *testing.T) {
	t.Helper()
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if strings.HasPrefix(upper, "AZURE_") || strings.HasPrefix(upper, "MSI_") || strings.HasPrefix(upper, "IDENTITY_") || upper == "IMDS_ENDPOINT" {
			t.Setenv(key, "")
			if err := os.Unsetenv(key); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("AZURE_TOKEN_CREDENTIALS", "WorkloadIdentityCredential")
}

func workloadEnv(t *testing.T) {
	t.Helper()
	isolateAzureEnv(t)
	file := filepath.Join(t.TempDir(), "assertion")
	if err := os.WriteFile(file, []byte("synthetic-assertion"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AZURE_CLIENT_ID", "env-client")
	t.Setenv("AZURE_TENANT_ID", "env-tenant")
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", file)
}

type tokenCredentialFunc func(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error)

func (f tokenCredentialFunc) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return f(ctx, opts)
}

type sdkTransportFunc func(*http.Request) (*http.Response, error)

func (f sdkTransportFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSDKMethodSelection(t *testing.T) {
	workloadEnv(t)
	f := func(method string, wantWorkload bool) {
		t.Helper()
		cfg := &SDConfig{SubscriptionID: "subscription", AuthenticationMethod: method}
		ac, err := newAPIConfig(cfg, "")
		if err != nil {
			t.Fatalf("cannot construct %s: %s", method, err)
		}
		ac.c.Stop()
		cred, err := newSDKCredential(cfg, policy.ClientOptions{})
		if err != nil {
			t.Fatal("cannot create SDK credential")
		}
		_, gotWorkload := cred.(*azidentity.WorkloadIdentityCredential)
		_, gotDefault := cred.(*azidentity.DefaultAzureCredential)
		if gotWorkload != wantWorkload || gotDefault == wantWorkload {
			t.Fatalf("unexpected credential type for %s", method)
		}
	}
	f("WorkloadIdentity", true)
	f("wOrKlOaDiDeNtItY", true)
	f("SDK", false)
	f("sDk", false)
}

func TestSDKConstructionErrors(t *testing.T) {
	isolateAzureEnv(t)
	f := func(cfg SDConfig, want string) {
		t.Helper()
		ac, err := newAPIConfig(&cfg, "")
		if err == nil {
			ac.c.Stop()
			t.Fatal("expected construction error")
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("unexpected error: %s; want %s", err, want)
		}
	}
	f(SDConfig{AuthenticationMethod: "SDK"}, "subscription_id")
	f(SDConfig{SubscriptionID: "s"}, "tenant_id")
	f(SDConfig{SubscriptionID: "s", AuthenticationMethod: "oAuTh", TenantID: "t"}, "client_id")
	f(SDConfig{SubscriptionID: "s", TenantID: "t", ClientID: "c"}, "client_secret")
	f(SDConfig{SubscriptionID: "s", AuthenticationMethod: "unknown"}, "WorkloadIdentity")
	f(SDConfig{SubscriptionID: "s", AuthenticationMethod: "WorkloadIdentity"}, "cannot create Azure SDK credential")
	t.Setenv("AZURE_TOKEN_CREDENTIALS", "synthetic-invalid-value")
	f(SDConfig{SubscriptionID: "s", AuthenticationMethod: "SDK"}, "cannot create Azure SDK credential")
}

func TestSDKCloudAndTenant(t *testing.T) {
	workloadEnv(t)
	t.Setenv("AZURE_AUTHORITY_HOST", "https://ignored.invalid")
	f := func(method, cloudName, tenant, configuredTenant string, useEnvironment bool) {
		t.Helper()
		env, err := getCloudEnvByName(cloudName)
		if err != nil {
			t.Fatal(err)
		}
		cfg := &SDConfig{AuthenticationMethod: method, TenantID: configuredTenant, ClientID: "ignored-client", ClientSecret: promauth.NewSecret("ignored-secret")}
		var tokenRequests int
		transport := sdkTransportFunc(func(req *http.Request) (*http.Response, error) {
			var data any
			switch {
			case strings.Contains(req.URL.Path, "discovery/instance"):
				host := strings.TrimPrefix(env.ActiveDirectoryEndpoint, "https://")
				data = map[string]any{"tenant_discovery_endpoint": env.ActiveDirectoryEndpoint + "/" + tenant + "/v2.0/.well-known/openid-configuration", "metadata": []any{map[string]any{"preferred_network": host, "preferred_cache": host, "aliases": []string{host}}}}
			case strings.HasSuffix(req.URL.Path, "/.well-known/openid-configuration"):
				if !strings.HasPrefix(req.URL.String(), env.ActiveDirectoryEndpoint+"/"+tenant+"/") {
					t.Error("wrong authority or tenant in metadata request")
				}
				data = map[string]string{"issuer": env.ActiveDirectoryEndpoint + "/" + tenant + "/v2.0", "authorization_endpoint": env.ActiveDirectoryEndpoint + "/" + tenant + "/oauth2/v2.0/authorize", "token_endpoint": env.ActiveDirectoryEndpoint + "/" + tenant + "/oauth2/v2.0/token"}
			case strings.HasSuffix(req.URL.Path, "/oauth2/v2.0/token"):
				tokenRequests++
				if req.URL.String() != env.ActiveDirectoryEndpoint+"/"+tenant+"/oauth2/v2.0/token" {
					t.Error("wrong token authority or tenant")
				}
				if err := req.ParseForm(); err != nil {
					t.Fatal("cannot parse synthetic request")
				}
				if req.Form.Get("client_id") != "env-client" || !slices.Contains(strings.Fields(req.Form.Get("scope")), env.ResourceManagerEndpoint+"/.default") {
					t.Error("wrong client or scope")
				}
				if useEnvironment {
					if req.Form.Get("client_secret") != "env-secret" {
						t.Error("environment credential did not use environment secret")
					}
				} else if req.Form.Get("client_assertion") != "synthetic-assertion" {
					t.Error("workload assertion missing")
				}
				data = map[string]any{"access_token": "synthetic-access", "expires_in": 3600, "token_type": "Bearer"}
			default:
				t.Fatal("unexpected SDK request; network is disabled")
			}
			b, err := json.Marshal(data)
			if err != nil {
				t.Fatal(err)
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(string(b))), Request: req}, nil
		})
		cred, err := newSDKCredential(cfg, policy.ClientOptions{Cloud: cloud.Configuration{ActiveDirectoryAuthorityHost: env.ActiveDirectoryEndpoint}, Transport: transport})
		if err != nil {
			t.Fatal("cannot construct credential")
		}
		refresh := sdkRefreshTokenFunc(context.Background(), cred, env.ResourceManagerEndpoint+"/.default")
		for i := 0; i < 2; i++ {
			token, duration, err := refresh()
			if err != nil || token != "synthetic-access" || duration < 59*time.Minute || duration > time.Hour {
				t.Fatalf("unexpected SDK refresh result (error=%v)", err)
			}
		}
		if tokenRequests != 1 {
			t.Fatalf("SDK cache not reused: %d requests", tokenRequests)
		}
	}
	for name := range cloudEnvironments {
		f("WorkloadIdentity", name, "env-tenant", "configured-tenant", false)
	}
	f("SDK", "AzurePublicCloud", "configured-tenant", "configured-tenant", false)
	f("SDK", "AzurePublicCloud", "env-tenant", "", false)
	t.Setenv("AZURE_TOKEN_CREDENTIALS", "EnvironmentCredential")
	t.Setenv("AZURE_CLIENT_SECRET", "env-secret")
	f("SDK", "AzurePublicCloud", "env-tenant", "configured-tenant", true)
}

func TestSDKRefreshCacheAndErrors(t *testing.T) {
	var calls atomic.Int32
	var fail atomic.Bool
	cred := tokenCredentialFunc(func(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
		calls.Add(1)
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > discoveryutil.DefaultClientReadTimeout || len(opts.Scopes) != 1 || opts.Scopes[0] != "resource/.default" {
			t.Error("missing deadline or incorrect scope")
		}
		if fail.Load() {
			return azcore.AccessToken{}, errors.New("synthetic-assertion synthetic-access")
		}
		return azcore.AccessToken{Token: "synthetic-access", ExpiresOn: time.Now().Add(time.Hour)}, nil
	})
	ac := &apiConfig{refreshToken: sdkRefreshTokenFunc(context.Background(), cred, "resource/.default")}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ac.mustGetAuthToken() != "synthetic-access" {
				t.Error("token not returned")
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("token cache did not serialize refresh")
	}
	ac.tokenExpireDeadline = time.Now().Add(20 * time.Second)
	if ac.mustGetAuthToken() == "" || calls.Load() != 2 {
		t.Fatal("near-expiry token not refreshed")
	}
	fail.Store(true)
	ac.tokenExpireDeadline = time.Now().Add(-time.Second)
	if ac.mustGetAuthToken() != "" {
		t.Fatal("stale token returned after refresh error")
	}
	_, _, err := ac.refreshToken()
	if err == nil || strings.Contains(err.Error(), "synthetic-") {
		t.Fatal("SDK error not sanitized")
	}
	fail.Store(false)
	if ac.mustGetAuthToken() == "" {
		t.Fatal("refresh did not recover")
	}
	f := func(token azcore.AccessToken) {
		t.Helper()
		refresh := sdkRefreshTokenFunc(context.Background(), tokenCredentialFunc(func(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) { return token, nil }), "scope")
		if value, _, err := refresh(); err == nil || value != "" {
			t.Fatal("invalid token accepted")
		}
	}
	f(azcore.AccessToken{ExpiresOn: time.Now().Add(time.Hour)})
	f(azcore.AccessToken{Token: "synthetic-access", ExpiresOn: time.Now().Add(-time.Second)})
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		if !errors.Is(sdkAuthError("failure", cause), cause) {
			t.Fatal("context error identity lost")
		}
	}
}

func TestSDKRefreshTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		cred := tokenCredentialFunc(func(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
			<-ctx.Done()
			return azcore.AccessToken{}, ctx.Err()
		})
		start := time.Now()
		refresh := sdkRefreshTokenFunc(context.Background(), cred, "scope")
		_, _, err := refresh()
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) != discoveryutil.DefaultClientReadTimeout {
			t.Fatal("SDK refresh timeout not enforced")
		}
	})
}

func TestSDKDiscoveryIntegration(t *testing.T) {
	workloadEnv(t)
	var tokenCalls, apiCalls atomic.Int32
	var serverURL string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Discovery-Test") != "present" {
			t.Error("configured HTTP header missing")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			fmt.Fprintf(w, `{"issuer":%q,"authorization_endpoint":%q,"token_endpoint":%q}`, serverURL+"/env-tenant/v2.0", serverURL+"/env-tenant/oauth2/v2.0/authorize", serverURL+"/env-tenant/oauth2/v2.0/token")
		case strings.HasSuffix(r.URL.Path, "/oauth2/v2.0/token"):
			tokenCalls.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Error("cannot parse synthetic assertion request")
			}
			if r.Form.Get("client_assertion") != "synthetic-assertion" || !slices.Contains(strings.Fields(r.Form.Get("scope")), serverURL+"/.default") {
				t.Error("wrong assertion or resource scope")
			}
			fmt.Fprint(w, `{"access_token":"synthetic-access","expires_in":3600,"token_type":"Bearer"}`)
		case strings.HasPrefix(r.URL.Path, "/subscriptions/"):
			apiCalls.Add(1)
			if r.Header.Get("Authorization") != "Bearer synthetic-access" {
				t.Error("discovery authorization missing")
			}
			fmt.Fprint(w, `{"value":[]}`)
		default:
			t.Error("unexpected loopback request")
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer server.Close()
	serverURL = server.URL
	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(dir, "cloud.json")
	data, err := json.Marshal(cloudEnvironmentEndpoints{ActiveDirectoryEndpoint: server.URL, ResourceManagerEndpoint: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AZURE_ENVIRONMENT_FILEPATH", envFile)
	cfg := &SDConfig{SubscriptionID: "synthetic-subscription", Environment: "AzureStackCloud", AuthenticationMethod: "WorkloadIdentity", HTTPClientConfig: promauth.HTTPClientConfig{TLSConfig: &promauth.TLSConfig{CAFile: caFile}, Headers: []string{"X-Discovery-Test: present"}}}
	defer cfg.MustStop()
	for i := 0; i < 2; i++ {
		labels, err := cfg.GetLabels("")
		if err != nil || len(labels) != 0 {
			t.Fatalf("discovery failed: %v", err)
		}
	}
	if tokenCalls.Load() != 1 || apiCalls.Load() != 4 {
		t.Fatalf("unexpected request counts: token=%d api=%d", tokenCalls.Load(), apiCalls.Load())
	}
	ac, err := getAPIConfig(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	ac.refreshToken = sdkRefreshTokenFunc(ac.c.Context(), tokenCredentialFunc(func(ctx context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
		close(started)
		<-ctx.Done()
		return azcore.AccessToken{}, ctx.Err()
	}), "scope")
	done := make(chan error, 1)
	go func() { _, _, err := ac.refreshToken(); done <- err }()
	<-started
	cfg.MustStop()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("stop did not cancel refresh")
		}
	case <-time.After(time.Second):
		t.Fatal("refresh blocked after stop")
	}
}

func TestSDKTokenErrors(t *testing.T) {
	workloadEnv(t)
	f := func(status int, body string, missingFile bool) {
		t.Helper()
		var requests int
		transport := sdkTransportFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			code, data := status, body
			if strings.HasSuffix(req.URL.Path, "/.well-known/openid-configuration") {
				code = http.StatusOK
				data = `{"issuer":"https://login.microsoftonline.com/env-tenant/v2.0","authorization_endpoint":"https://login.microsoftonline.com/env-tenant/oauth2/v2.0/authorize","token_endpoint":"https://login.microsoftonline.com/env-tenant/oauth2/v2.0/token"}`
			}
			return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(data)), Request: req}, nil
		})
		if missingFile {
			t.Setenv("AZURE_FEDERATED_TOKEN_FILE", filepath.Join(t.TempDir(), "missing"))
		}
		cred, err := newSDKCredential(&SDConfig{AuthenticationMethod: "WorkloadIdentity"}, policy.ClientOptions{Transport: transport})
		if err != nil {
			t.Fatal("unexpected construction error")
		}
		refresh := sdkRefreshTokenFunc(context.Background(), cred, "https://management.azure.com/.default")
		token, _, err := refresh()
		if err == nil || token != "" || strings.Contains(err.Error(), "synthetic-") {
			t.Fatal("SDK error response was accepted or not sanitized")
		}
		if !missingFile && requests == 0 {
			t.Fatal("test did not exercise SDK HTTP errors")
		}
	}
	f(http.StatusUnauthorized, `{"error":"invalid_client","error_description":"synthetic-assertion synthetic-access"}`, false)
	f(http.StatusOK, `invalid JSON synthetic-access`, false)
	f(http.StatusOK, `{}`, true)
}

func TestSDKHTTPOptions(t *testing.T) {
	var proxyCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalls.Add(1)
		if r.Header.Get("X-Test") != "header" || r.Header.Get("Proxy-Authorization") != "Bearer synthetic-proxy" || r.Header.Get("Authorization") != "Bearer synthetic-http" {
			t.Error("configured auth or HTTP header missing")
		}
		w.Header().Set("Location", "http://unused.invalid/redirected")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	follow := false
	cfg := &SDConfig{ProxyURL: proxy.MustNewURL(server.URL), HTTPClientConfig: promauth.HTTPClientConfig{Headers: []string{"X-Test: header"}, BearerToken: promauth.NewSecret("synthetic-http"), FollowRedirects: &follow}, ProxyClientConfig: promauth.ProxyClientConfig{BearerToken: promauth.NewSecret("synthetic-proxy")}}
	ac, err := cfg.HTTPClientConfig.NewConfig("")
	if err != nil {
		t.Fatal(err)
	}
	proxyAC, err := cfg.ProxyClientConfig.NewConfig("")
	if err != nil {
		t.Fatal(err)
	}
	client := newSDKHTTPClient(cfg, ac, proxyAC)
	req, err := http.NewRequest(http.MethodGet, "http://unused.invalid/token", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal("proxy request failed")
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusFound || proxyCalls.Load() != 1 || client.client.Timeout != discoveryutil.DefaultClientReadTimeout {
		t.Fatal("redirect or timeout option not honored")
	}
}
