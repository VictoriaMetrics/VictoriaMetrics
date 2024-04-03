package promauth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v2"
)

func TestOptionsNewConfigFailure(t *testing.T) {
	f := func(yamlConfig string) {
		t.Helper()

		var hcc HTTPClientConfig
		if err := yaml.UnmarshalStrict([]byte(yamlConfig), &hcc); err != nil {
			t.Fatalf("cannot parse: %s", err)
		}
		cfg, err := hcc.NewConfig("")
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if cfg != nil {
			t.Fatalf("expecting nil cfg; got %s", cfg.String())
		}
	}

	// authorization: both credentials and credentials_file are set
	f(`
authorization:
  credentials: foo-bar
  credentials_file: testdata/test_secretfile.txt
`)

	// basic_auth: both authorization and basic_auth are set
	f(`
authorization:
  credentials: foo-bar
basic_auth:
  username: user
  password: pass
`)

	// basic_auth: missing username
	f(`
basic_auth:
  password: pass
`)

	// basic_auth: both username and username_file are set
	f(`
basic_auth:
  username: foo
  username_file: testdata/test_secretfile.txt
`)

	// basic_auth: both password and password_file are set
	f(`
basic_auth:
  username: user
  password: pass
  password_file: testdata/test_secretfile.txt
`)

	// bearer_token: both authorization and bearer_token are set
	f(`
authorization:
  credentials: foo-bar
bearer_token: bearer-aaa
`)

	// bearer_token: both basic_auth and bearer_token are set
	f(`
bearer_token: bearer-aaa
basic_auth:
  username: user
  password: pass
`)

	// bearer_token_file: both authorization and bearer_token_file are set
	f(`
authorization:
  credentials: foo-bar
bearer_token_file: testdata/test_secretfile.txt
`)

	// bearer_token_file: both basic_auth and bearer_token_file are set
	f(`
bearer_token_file: testdata/test_secretfile.txt
basic_auth:
  username: user
  password: pass
`)

	// both bearer_token_file and bearer_token are set
	f(`
bearer_token_file: testdata/test_secretfile.txt
bearer_token: foo-bar
`)

	// oauth2: both oauth2 and authorization are set
	f(`
authorization:
  credentials: foo-bar
oauth2:
  client_id: some-id
  client_secret: some-secret
  token_url: http://some-url
`)

	// oauth2: both oauth2 and basic_auth are set
	f(`
basic_auth:
  username: user
  password: pass
oauth2:
  client_id: some-id
  client_secret: some-secret
  token_url: http://some-url
`)

	// oauth2: both oauth2 and bearer_token are set
	f(`
bearer_token: foo-bar
oauth2:
  client_id: some-id
  client_secret: some-secret
  token_url: http://some-url
`)

	// oauth2: both oauth2 and bearer_token_file are set
	f(`
bearer_token_file: testdata/test_secretfile.txt
oauth2:
  client_id: some-id
  client_secret: some-secret
  token_url: http://some-url
`)

	// oauth2: missing client_id
	f(`
oauth2:
  client_secret: some-secret
  token_url: http://some-url
`)

	// oauth2: invalid inline tls config
	f(`
oauth2:
  client_id: some-id
  client_secret: some-secret
  token_url: http://some-url
  tls_config:
    key: foobar
    cert: baz
`)

	// oauth2: invalid ca at tls_config
	f(`
oauth2:
  client_id: some-id
  client_secret: some-secret
  token_url: http://some-url
  tls_config:
    ca: foobar
`)

	// oauth2: invalid min_version at tls_config
	f(`
oauth2:
  client_id: some-id
  client_secret: some-secret
  token_url: http://some-url
  tls_config:
    min_version: foobar
`)

	// oauth2: invalid proxy_url
	f(`
oauth2:
  client_id: some-id
  client_secret: some-secret
  token_url: http://some-url
  proxy_url: ":invalid-proxy-url"
`)

	// tls_config: invalid ca
	f(`
tls_config:
  ca: foobar
`)

	// invalid headers
	f(`
headers:
- foobar
`)

}

func TestOauth2ConfigParseFailure(t *testing.T) {
	f := func(yamlConfig string) {
		t.Helper()

		var cfg OAuth2Config
		if err := yaml.UnmarshalStrict([]byte(yamlConfig), &cfg); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// invalid yaml
	f("afdsfds")

	// unknown fields
	f("foobar: baz")
}

func TestOauth2ConfigValidateFailure(t *testing.T) {
	f := func(yamlConfig string) {
		t.Helper()

		var cfg OAuth2Config
		if err := yaml.UnmarshalStrict([]byte(yamlConfig), &cfg); err != nil {
			t.Fatalf("cannot unmarshal config: %s", err)
		}
		if err := cfg.validate(); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// emtpy client_id
	f(`
client_secret: some-secret
token_url: http://some-url
`)

	// missing client_secret and client_secret_file
	f(`
client_id: some-id
token_url: http://some-url/
`)

	// client_secret and client_secret_file are set simultaneously
	f(`
client_id: some-id
client_secret: some-secret
client_secret_file: testdata/test_secretfile.txt
token_url: http://some-url/
`)

	// missing token_url
	f(`
client_id: some-id
client_secret: some-secret
`)
}

func TestOauth2ConfigValidateSuccess(t *testing.T) {
	f := func(yamlConfig string) {
		t.Helper()

		var cfg OAuth2Config
		if err := yaml.UnmarshalStrict([]byte(yamlConfig), &cfg); err != nil {
			t.Fatalf("cannot parse: %s", err)
		}
		if err := cfg.validate(); err != nil {
			t.Fatalf("cannot validate: %s", err)
		}
	}

	f(`
client_id: some-id
client_secret: some-secret
token_url: http://some-url/
proxy_url: http://some-proxy/abc
scopes: [read, write, execute]
endpoint_params:
  foo: bar
  abc: def
tls_config:
  insecure_skip_verify: true
`)
}

func TestConfigGetAuthHeaderFailure(t *testing.T) {
	f := func(yamlConfig string) {
		t.Helper()

		var hcc HTTPClientConfig
		if err := yaml.UnmarshalStrict([]byte(yamlConfig), &hcc); err != nil {
			t.Fatalf("cannot parse: %s", err)
		}
		cfg, err := hcc.NewConfig("")
		if err != nil {
			t.Fatalf("cannot initialize config: %s", err)
		}

		// Verify that GetAuthHeader() returns error
		ah, err := cfg.GetAuthHeader()
		if err == nil {
			t.Fatalf("expecting non-nil error from GetAuthHeader()")
		}
		if ah != "" {
			t.Fatalf("expecting empty auth header; got %q", ah)
		}

		// Verify that SetHeaders() returns error
		req, err := http.NewRequest(http.MethodGet, "http://foo", nil)
		if err != nil {
			t.Fatalf("unexpected error in http.NewRequest: %s", err)
		}
		if err := cfg.SetHeaders(req, true); err == nil {
			t.Fatalf("expecting non-nil error from SetHeaders()")
		}

		// Verify that the tls cert cannot be loaded properly if it exists
		if f := cfg.getTLSCertCached; f != nil {
			cert, err := f(nil)
			if err == nil {
				t.Fatalf("expecting non-nil error in getTLSCertCached()")
			}
			if cert != nil {
				t.Fatalf("expecting nil cert from getTLSCertCached()")
			}
		}
	}

	// oauth2 with invalid proxy_url
	f(`
oauth2:
  client_id: some-id
  client_secret: some-secret
  token_url: http://some-url
  proxy_url: invalid-proxy-url
`)

	// oauth2 with non-existing client_secret_file
	f(`
oauth2:
  client_id: some-id
  client_secret_file: non-existing-file
  token_url: http://some-url
`)

	// non-existing root ca file for oauth2
	f(`
oauth2:
  client_id: some-id
  client_secret: some-secret
  token_url: http://some-url
  tls_config:
    ca_file: non-existing-file
`)

	// basic auth via non-existing username file
	f(`
basic_auth:
  username_file: non-existing-file
  password: foobar
`)

	// basic auth via non-existing password file
	f(`
basic_auth:
  username: user
  password_file: non-existing-file
`)

	// bearer token via non-existing file
	f(`
bearer_token_file: non-existing-file
`)

	// authorization creds via non-existing file
	f(`
authorization:
  type: foobar
  credentials_file: non-existing-file
`)
}

func TestConfigGetAuthHeaderSuccess(t *testing.T) {
	f := func(yamlConfig string, ahExpected string) {
		t.Helper()

		var hcc HTTPClientConfig
		if err := yaml.UnmarshalStrict([]byte(yamlConfig), &hcc); err != nil {
			t.Fatalf("cannot unmarshal config: %s", err)
		}
		if hcc.OAuth2 != nil {
			if hcc.OAuth2.TokenURL != "replace-with-mock-url" {
				t.Fatalf("unexpected token_url: %q; want `replace-with-mock-url`", hcc.OAuth2.TokenURL)
			}
			r := http.NewServeMux()
			r.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"access_token":"test-oauth2-token","token_type": "Bearer"}`))
			})
			mock := httptest.NewServer(r)
			hcc.OAuth2.TokenURL = mock.URL
		}
		cfg, err := hcc.NewConfig("")
		if err != nil {
			t.Fatalf("cannot initialize config: %s", err)
		}

		// Verify that cfg.String() returns non-empty value
		cfgString := cfg.String()
		if cfgString == "" {
			t.Fatalf("unexpected empty result from Config.String")
		}

		// Check that GetAuthHeader() returns the correct header
		ah, err := cfg.GetAuthHeader()
		if err != nil {
			t.Fatalf("unexpected auth header; got %q; want %q", ah, ahExpected)
		}

		// Make sure that cfg.SetHeaders() properly set Authorization header
		req, err := http.NewRequest(http.MethodGet, "http://foo", nil)
		if err != nil {
			t.Fatalf("unexpected error in http.NewRequest: %s", err)
		}
		if err := cfg.SetHeaders(req, true); err != nil {
			t.Fatalf("unexpected error in SetHeaders(): %s", err)
		}
		ah = req.Header.Get("Authorization")
		if ah != ahExpected {
			t.Fatalf("unexpected auth header from net/http request; got %q; want %q", ah, ahExpected)
		}
	}

	// Zero config
	f(``, "")

	// no auth config, non-zero tls config
	f(`
tls_config:
  insecure_skip_verify: true
`, "")

	// no auth config, tls_config with non-existing files
	f(`
tls_config:
  key_file: non-existing-file
  cert_file: non-existing-file
`, "")

	// no auth config, tls_config with non-existing ca file
	f(`
tls_config:
  ca_file: non-existing-file
`, "")

	// inline oauth2 config
	f(`
oauth2:
  client_id: some-id
  client_secret: some-secret
  endpoint_params:
    foo: bar
  scopes: [foo, bar]
  token_url: replace-with-mock-url
`, "Bearer test-oauth2-token")

	// oauth2 config with secrets in the file
	f(`
oauth2:
  client_id: some-id
  client_secret_file: testdata/test_secretfile.txt
  token_url: replace-with-mock-url
`, "Bearer test-oauth2-token")

	// inline basic auth
	f(`
basic_auth:
  username: user
  password: password
`, "Basic dXNlcjpwYXNzd29yZA==")

	// basic auth via username file
	f(`
basic_auth:
  username_file: testdata/test_secretfile.txt
`, "Basic c2VjcmV0LWNvbnRlbnQ6")

	// basic auth via password file
	f(`
basic_auth:
  username: user
  password_file: testdata/test_secretfile.txt
`, "Basic dXNlcjpzZWNyZXQtY29udGVudA==")

	// basic auth via username file and password file
	f(`
basic_auth:
  username_file: testdata/test_secretfile.txt
  password_file: testdata/test_secretfile.txt
`, "Basic c2VjcmV0LWNvbnRlbnQ6c2VjcmV0LWNvbnRlbnQ=")

	// inline authorization config
	f(`
authorization:
  type: My-Super-Auth
  credentials: some-password
`, "My-Super-Auth some-password")

	// authorization config via file
	f(`
authorization:
  type: Foo
  credentials_file: testdata/test_secretfile.txt
`, "Foo secret-content")

	// inline bearer token
	f(`
bearer_token: some-token
`, "Bearer some-token")

	// bearer token via file
	f(`
bearer_token_file: testdata/test_secretfile.txt
`, "Bearer secret-content")
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
	f([]string{"Foo: bar"})
	f([]string{"Foo: bar", "A-B-C: d-e-f"})
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
		req, err := http.NewRequest(http.MethodGet, "http://foo", nil)
		if err != nil {
			t.Fatalf("unexpected error in http.NewRequest: %s", err)
		}
		result := c.HeadersNoAuthString()
		if result != resultExpected {
			t.Fatalf("unexpected result from HeadersNoAuthString; got\n%s\nwant\n%s", result, resultExpected)
		}
		if err := c.SetHeaders(req, false); err != nil {
			t.Fatalf("unexpected error in SetHeaders(): %s", err)
		}
		for _, h := range headersParsed {
			v := req.Header.Get(h.key)
			if v != h.value {
				t.Fatalf("unexpected value for net/http header %q; got %q; want %q", h.key, v, h.value)
			}
		}
	}
	f(nil, "")
	f([]string{"foo: bar"}, "Foo: bar\r\n")
	f([]string{"Foo-Bar: Baz s:sdf", "A:b", "X-Forwarded-For: A-B:c"}, "Foo-Bar: Baz s:sdf\r\nA: b\r\nX-Forwarded-For: A-B:c\r\n")
}

func TestTLSConfigWithCertificatesFilesUpdate(t *testing.T) {
	// Generate and save a self-signed CA certificate and a certificate signed by the CA
	caPEM, certPEM, keyPEM := mustGenerateCertificates()
	_ = os.WriteFile("testdata/ca.pem", caPEM, 0644)

	defer func() {
		_ = os.Remove("testdata/ca.pem")
	}()

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("cannot load generated certificate: %s", err)
	}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}

	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	s.TLS = tlsConfig
	s.StartTLS()
	serverURL, err := url.Parse(s.URL)
	if err != nil {
		t.Fatalf("unexpected error when parsing url=%q: %s", s.URL, err)
	}

	opts := Options{
		TLSConfig: &TLSConfig{
			CAFile: "testdata/ca.pem",
		},
	}
	ac, err := opts.NewConfig()
	if err != nil {
		t.Fatalf("unexpected error when parsing config: %s", err)
	}

	client := http.Client{
		Transport: ac.NewRoundTripper(&http.Transport{}),
	}

	resp, err := client.Do(&http.Request{
		Method: http.MethodGet,
		URL:    serverURL,
	})
	if err != nil {
		t.Fatalf("unexpected error when making request: %s", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status code %d; got %d", http.StatusOK, resp.StatusCode)
	}

	// Update CA file with new CA and get config
	ca2PEM, _, _ := mustGenerateCertificates()
	_ = os.WriteFile("testdata/ca.pem", ca2PEM, 0644)

	// Wait for cert cache expiration
	time.Sleep(2 * time.Second)

	_, err = client.Do(&http.Request{
		Method: http.MethodGet,
		URL:    serverURL,
	})
	if err == nil {
		t.Fatal("expected TLS verification error, got nil")
	}
}

func mustGenerateCertificates() ([]byte, []byte, []byte) {
	// Small key size for faster tests
	const testCertificateBits = 1024

	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2024),
		Subject: pkix.Name{
			Organization: []string{"Test CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caPrivKey, err := rsa.GenerateKey(rand.Reader, testCertificateBits)
	if err != nil {
		panic(fmt.Errorf("cannot generate CA private key: %s", err))
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		panic(fmt.Errorf("cannot create CA certificate: %s", err))
	}
	caPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2020),
		Subject: pkix.Name{
			Organization: []string{"Test Cert"},
		},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  false,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	key, err := rsa.GenerateKey(rand.Reader, testCertificateBits)
	if err != nil {
		panic(fmt.Errorf("cannot generate certificate private key: %s", err))
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &key.PublicKey, caPrivKey)
	if err != nil {
		panic(fmt.Errorf("cannot generate certificate: %s", err))
	}
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return caPEM, certPEM, keyPEM
}
