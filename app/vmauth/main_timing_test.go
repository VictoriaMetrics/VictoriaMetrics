package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"
)

func BenchmarkJWTRequestHandler(b *testing.B) {
	// Generate RSA key pair for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatalf("cannot generate RSA key: %s", err)
	}

	// Generate public key PEM
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		b.Fatalf("cannot marshal public key: %s", err)
	}
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	genToken := func(t *testing.B, body map[string]any, valid bool) string {
		t.Helper()

		headerJSON, err := json.Marshal(map[string]any{
			"alg": "RS256",
			"typ": "JWT",
		})
		if err != nil {
			t.Fatalf("cannot marshal header: %s", err)
		}
		headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

		bodyJSON, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("cannot marshal body: %s", err)
		}
		bodyB64 := base64.RawURLEncoding.EncodeToString(bodyJSON)

		payload := headerB64 + "." + bodyB64

		var signatureB64 string
		if valid {
			// Create real RSA signature
			hash := crypto.SHA256
			h := hash.New()
			h.Write([]byte(payload))
			digest := h.Sum(nil)

			signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, hash, digest)
			if err != nil {
				t.Fatalf("cannot sign token: %s", err)
			}
			signatureB64 = base64.RawURLEncoding.EncodeToString(signature)
		} else {
			signatureB64 = base64.RawURLEncoding.EncodeToString([]byte("invalid_signature"))
		}

		return payload + "." + signatureB64
	}

	f := func(name string, cfgStr string, r *http.Request, responseExpected string) {
		b.Helper()

		b.ReportAllocs()
		b.ResetTimer()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte("path: " + r.URL.Path + "\n")); err != nil {
				panic(fmt.Errorf("cannot write response: %w", err))
			}
			if _, err := w.Write([]byte("query:\n")); err != nil {
				panic(fmt.Errorf("cannot write response: %w", err))
			}
			names := make([]string, 0, len(r.URL.Query()))
			query := r.URL.Query()
			for n := range query {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				for _, v := range query[n] {
					if _, err := w.Write([]byte("    " + n + "=" + v + "\n")); err != nil {
						panic(fmt.Errorf("cannot write response: %w", err))
					}
				}
			}

			if _, err := w.Write([]byte("headers:\n")); err != nil {
				panic(fmt.Errorf("cannot write response: %w", err))
			}
			if v := r.Header.Get(`AccountID`); v != "" {
				if _, err := w.Write([]byte(`    AccountID=` + v + "\n")); err != nil {
					panic(fmt.Errorf("cannot write response: %w", err))
				}
			}
			if v := r.Header.Get(`ProjectID`); v != "" {
				if _, err := w.Write([]byte(`    ProjectID=` + v + "\n")); err != nil {
					panic(fmt.Errorf("cannot write response: %w", err))
				}
			}
		}))
		defer ts.Close()

		cfgStr = strings.ReplaceAll(cfgStr, "{BACKEND}", ts.URL)
		responseExpected = strings.ReplaceAll(responseExpected, "{BACKEND}", ts.URL)

		cfgOrigP := authConfigData.Load()
		if _, err := reloadAuthConfigData([]byte(cfgStr)); err != nil {
			b.Fatalf("cannot load config data: %s", err)
		}
		defer func() {
			cfgOrig := []byte("unauthorized_user:\n  url_prefix: http://foo/bar")
			if cfgOrigP != nil {
				cfgOrig = *cfgOrigP
			}
			_, err := reloadAuthConfigData(cfgOrig)
			if err != nil {
				b.Fatalf("cannot load the original config: %s", err)
			}
		}()
		responseExpected = strings.TrimSpace(responseExpected)

		b.Run(name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					w := &fakeResponseWriter{}
					if !requestHandlerWithInternalRoutes(w, r) {
						b.Fatalf("unexpected false is returned from requestHandler")
					}

					response := w.getResponse()
					response = strings.ReplaceAll(response, "\r\n", "\n")
					response = strings.TrimSpace(response)
					if response != responseExpected {
						b.Fatalf("unexpected response\ngot\n%s\nwant\n%s", response, responseExpected)
					}
				}
			})
		})
	}

	simpleCfgStr := fmt.Sprintf(`
users:
- jwt:
    public_keys:
    - %q
  url_prefix: {BACKEND}/foo`, string(publicKeyPEM))
	noVMAccessClaimToken := genToken(b, nil, true)
	expiredToken := genToken(b, map[string]any{
		"exp":       10,
		"vm_access": map[string]any{},
	}, true)

	fullToken := genToken(b, map[string]any{
		"exp":   time.Now().Add(10 * time.Minute).Unix(),
		"scope": "email id",
		"vm_access": map[string]any{
			"extra_labels": map[string]string{
				"label":  "value1",
				"label2": "value3",
			},
			"extra_filters":      []string{"stream_filter1", "stream_filter2"},
			"metrics_account_id": 123,
			"metrics_project_id": 234,
			"metrics_extra_labels": []string{
				"label1=value1",
				"label2=value2",
			},
			"metrics_extra_filters": []string{
				`{label3="value3"}`,
				`{label4="value4"}`,
			},
			"logs_account_id": 345,
			"logs_project_id": 456,
			"logs_extra_filters": []string{
				`{"namespace":"my-app","env":"prod"}`,
			},
			"logs_extra_stream_filters": []string{
				`{"team":"dev"}`,
			},
		},
	}, true)

	// tenant headers are overwritten if set as placeholders
	// extra_filters extra_stream_filters from vm_access claim merged with statically defined
	request := httptest.NewRequest(`GET`, "http://some-host.com/query", nil)
	request.Header.Set(`Authorization`, `Bearer `+fullToken)
	responseExpected := `
statusCode=200
path: /select/logsql/query
query:
    extra_filters=aStaticFilter
    extra_filters={"namespace":"my-app","env":"prod"}
    extra_stream_filters=aStaticStreamFilter
    extra_stream_filters={"team":"dev"}
headers:
    AccountID=345
    ProjectID=456`
	f("full_template",
		fmt.Sprintf(`
users:
- jwt:
    public_keys:
    - %q
  headers:
    - "AccountID: {{.LogsAccountID}}"
    - "ProjectID: {{.LogsProjectID}}"
  url_prefix: {BACKEND}/select/logsql/?extra_filters=aStaticFilter&extra_stream_filters=aStaticStreamFilter&extra_filters={{.LogsExtraFilters}}&extra_stream_filters={{.LogsExtraStreamFilters}}`, string(publicKeyPEM)),
		request,
		responseExpected,
	)

	// token without vm_access claim
	request = httptest.NewRequest(`GET`, "http://some-host.com/abc", nil)
	request.Header.Set(`Authorization`, `Bearer `+noVMAccessClaimToken)
	responseExpected = `
statusCode=401
Unauthorized`
	f("token_without_claim", simpleCfgStr, request, responseExpected)

	// expired token
	request = httptest.NewRequest(`GET`, "http://some-host.com/abc", nil)
	request.Header.Set(`Authorization`, `Bearer `+expiredToken)
	responseExpected = `
statusCode=401
Unauthorized`
	f("expired_token", simpleCfgStr, request, responseExpected)
}
