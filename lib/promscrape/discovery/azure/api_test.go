package azure

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func TestLegacyAuthentication(t *testing.T) {
	isolateAzureEnv(t)
	f := func(method, msiSecret, identityHeader string) {
		t.Helper()
		t.Setenv("MSI_SECRET", msiSecret)
		t.Setenv("IDENTITY_HEADER", identityHeader)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.EqualFold(method, "ManagedIdentity") {
				q := r.URL.Query()
				clientParam, version := "client_id", "2018-02-01"
				if msiSecret != "" {
					clientParam, version = "clientid", "2017-09-01"
				}
				if identityHeader != "" {
					clientParam, version = "client_id", "2019-08-01"
				}
				if r.Method != http.MethodGet || q.Get(clientParam) != "configured-client" || q.Get("api-version") != version || q.Get("resource") != "https://resource.invalid" {
					t.Error("legacy managed identity request changed")
				}
				if msiSecret == "" && r.Header.Get("Metadata") != "true" || msiSecret != "" && r.Header.Get("secret") != msiSecret {
					t.Error("legacy managed identity headers changed")
				}
			} else {
				// The legacy OAuth path doesn't set Content-Type; parse the body explicitly.
				r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				if err := r.ParseForm(); err != nil {
					t.Error("cannot parse legacy OAuth form")
				}
				if r.Method != http.MethodPost || r.URL.Path != "/configured-tenant/oauth2/token" || r.Form.Get("client_id") != "configured-client" || r.Form.Get("client_secret") != "synthetic-secret" || r.Form.Get("resource") != "https://resource.invalid" || r.Form.Get("grant_type") != "client_credentials" {
					t.Error("legacy OAuth request changed")
				}
			}
			fmt.Fprint(w, `{"access_token":"synthetic-legacy","expires_in":"3600"}`)
		}))
		defer server.Close()
		t.Setenv("MSI_ENDPOINT", server.URL+"/metadata/identity/oauth2/token")
		cfg := &SDConfig{AuthenticationMethod: method, TenantID: "configured-tenant", ClientID: "configured-client", ClientSecret: promauth.NewSecret("synthetic-secret")}
		ac, err := cfg.HTTPClientConfig.NewConfig("")
		if err != nil {
			t.Fatal(err)
		}
		proxyAC, err := cfg.ProxyClientConfig.NewConfig("")
		if err != nil {
			t.Fatal(err)
		}
		refresh, err := getRefreshTokenFunc(context.Background(), cfg, ac, proxyAC, &cloudEnvironmentEndpoints{ActiveDirectoryEndpoint: server.URL, ResourceManagerEndpoint: "https://resource.invalid"})
		if err != nil {
			t.Fatal(err)
		}
		token, expiry, err := refresh()
		if err != nil || token != "synthetic-legacy" || expiry != time.Hour {
			t.Fatal("legacy token response changed")
		}
	}
	f("", "", "")
	f("oAuTh", "", "")
	f("ManagedIdentity", "", "")
	f("mAnAgEdIdEnTiTy", "synthetic-msi", "")
	f("ManagedIdentity", "synthetic-msi", "synthetic-header")
}
