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

	f := func(name string, cfgStr string, r *http.Request, statusCodeExpected int) {
		b.Helper()

		b.ReportAllocs()
		b.ResetTimer()
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write([]byte("path: " + r.URL.Path + "\n")); err != nil {
				panic(fmt.Errorf("cannot write response: %w", err))
			}
		}))
		defer ts.Close()

		cfgStr = strings.ReplaceAll(cfgStr, "{BACKEND}", ts.URL)

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

		b.Run(name, func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()
			b.RunParallel(func(pb *testing.PB) {
				w := &fakeResponseWriter{}
				for pb.Next() {
					w.reset()
					if !requestHandlerWithInternalRoutes(w, r) {
						b.Fatalf("unexpected false is returned from requestHandler")
					}
					if w.statusCode != statusCodeExpected {
						b.Fatalf("unexpected response code (-%d;+%d)", statusCodeExpected, w.statusCode)
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
		http.StatusOK,
	)

	// token without vm_access claim
	request = httptest.NewRequest(`GET`, "http://some-host.com/abc", nil)
	request.Header.Set(`Authorization`, `Bearer `+noVMAccessClaimToken)
	f("token_without_claim", simpleCfgStr, request, http.StatusUnauthorized)

	// expired token
	request = httptest.NewRequest(`GET`, "http://some-host.com/abc", nil)
	request.Header.Set(`Authorization`, `Bearer `+expiredToken)
	f("expired_token", simpleCfgStr, request, http.StatusUnauthorized)
}
