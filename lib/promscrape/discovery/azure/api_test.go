package azure

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetRefreshTokenFuncWorkloadIdentity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" ||
			r.PostForm.Get("grant_type") != "client_credentials" ||
			r.PostForm.Get("client_assertion_type") != "urn:ietf:params:oauth:client-assertion-type:jwt-bearer" ||
			r.PostForm.Get("scope") != "https://management.azure.com/.default" {
			http.Error(w, fmt.Sprintf("unexpected request: %s %s", r.Method, r.PostForm), http.StatusBadRequest)
			return
		}
		accessToken := r.URL.Path + " " + r.PostForm.Get("client_id") + " " + r.PostForm.Get("client_assertion")
		fmt.Fprintf(w, `{"access_token":%q,"expires_in":3599}`, accessToken)
	}))
	defer srv.Close()

	tokenFile := filepath.Join(t.TempDir(), "token")
	t.Setenv("AZURE_TENANT_ID", "some-tenant")
	t.Setenv("AZURE_CLIENT_ID", "some-client")
	t.Setenv("AZURE_FEDERATED_TOKEN_FILE", tokenFile)
	env := &cloudEnvironmentEndpoints{
		ActiveDirectoryEndpoint: srv.URL,
		ResourceManagerEndpoint: "https://management.azure.com",
	}
	refreshToken, err := getRefreshTokenFunc(&SDConfig{AuthenticationMethod: "WorkloadIdentity"}, nil, nil, env)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	f := func(assertion, tokenExpected string) {
		t.Helper()
		if err := os.WriteFile(tokenFile, []byte(assertion), 0o600); err != nil {
			t.Fatalf("cannot write token file: %s", err)
		}
		token, expiresIn, err := refreshToken()
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if token != tokenExpected {
			t.Fatalf("unexpected token; got %q; want %q", token, tokenExpected)
		}
		if expiresIn != 3599*time.Second {
			t.Fatalf("unexpected expiresIn; got %s; want %s", expiresIn, 3599*time.Second)
		}
	}

	// The token file must be read on every refresh, since Kubernetes rotates it.
	f("assertion-1\n", "/some-tenant/oauth2/v2.0/token some-client assertion-1")
	f("assertion-2", "/some-tenant/oauth2/v2.0/token some-client assertion-2")
}

func TestGetRefreshTokenFuncWorkloadIdentityFailure(t *testing.T) {
	f := func(tenantID, clientID, tokenFile string) {
		t.Helper()
		t.Setenv("AZURE_TENANT_ID", tenantID)
		t.Setenv("AZURE_CLIENT_ID", clientID)
		t.Setenv("AZURE_FEDERATED_TOKEN_FILE", tokenFile)
		sdc := &SDConfig{AuthenticationMethod: "WorkloadIdentity"}
		if _, err := getRefreshTokenFunc(sdc, nil, nil, cloudEnvironments["AZURECLOUD"]); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	f("", "client", "/token")
	f("tenant", "", "/token")
	f("tenant", "client", "")
}

func TestParseTokenExpiry(t *testing.T) {
	f := func(data string, expiresInExpected int64) {
		t.Helper()
		var tr tokenResponse
		if err := json.Unmarshal([]byte(data), &tr); err != nil {
			t.Fatalf("cannot parse %q: %s", data, err)
		}
		expiresIn, err := parseTokenExpiry(tr)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if expiresIn != expiresInExpected {
			t.Fatalf("unexpected expiresIn; got %d; want %d", expiresIn, expiresInExpected)
		}
	}

	// v1 token response
	f(`{"access_token":"foo","expires_in":"3599"}`, 3599)

	// v2.0 token response
	f(`{"access_token":"foo","expires_in":3599}`, 3599)
}
