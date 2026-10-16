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
	"math/big"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jwt"
)

// tokenTester generates RSA key pairs and signed JWT tokens for testing.
type tokenTester struct {
	t            testing.TB
	privateKey   *rsa.PrivateKey
	PublicKeyPEM string
}

// newTokenTester creates a tokenTester with a freshly generated 2048-bit RSA key pair.
func newTokenTester(t testing.TB) *tokenTester {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("cannot generate RSA key: %s", err)
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("cannot marshal public key: %s", err)
	}
	publicKeyPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}))

	return &tokenTester{
		t:            t,
		privateKey:   privateKey,
		PublicKeyPEM: publicKeyPEM,
	}
}

// NewVerifierPool creates a VerifierPool from the test RSA public key.
func (jt *tokenTester) NewVerifierPool() *jwt.VerifierPool {
	jt.t.Helper()
	vp, err := jwt.NewVerifierPool([]any{&jt.privateKey.PublicKey})
	if err != nil {
		jt.t.Fatalf("cannot create verifier pool: %s", err)
	}
	return vp
}

// JWKS returns JWKS JSON containing the test RSA public key with the given kid.
func (jt *tokenTester) JWKS(kid string) string {
	nBytes := jt.privateKey.N.Bytes()
	eBytes := big.NewInt(int64(jt.privateKey.E)).Bytes()
	return fmt.Sprintf(`{"keys":[{"kty":"RSA","kid":%q,"n":%q,"e":%q}]}`,
		kid,
		base64.RawURLEncoding.EncodeToString(nBytes),
		base64.RawURLEncoding.EncodeToString(eBytes),
	)
}

// GenToken generates a signed JWT with the given body claims.
// If valid is false, the signature is invalid.
func (jt *tokenTester) GenToken(body map[string]any, valid bool) string {
	jt.t.Helper()
	return jt.GenTokenWithHeader(nil, body, valid)
}

// GenTokenWithHeader generates a signed JWT with the given extra header fields and body claims.
// If valid is false, the signature is invalid.
func (jt *tokenTester) GenTokenWithHeader(extraHeader, body map[string]any, valid bool) string {
	jt.t.Helper()

	header := map[string]any{
		"alg": "RS256",
		"typ": "JWT",
	}
	for k, v := range extraHeader {
		header[k] = v
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		jt.t.Fatalf("cannot marshal header: %s", err)
	}
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		jt.t.Fatalf("cannot marshal body: %s", err)
	}
	bodyB64 := base64.RawURLEncoding.EncodeToString(bodyJSON)

	payload := headerB64 + "." + bodyB64

	var signatureB64 string
	if valid {
		hash := crypto.SHA256
		h := hash.New()
		h.Write([]byte(payload))
		digest := h.Sum(nil)

		signature, err := rsa.SignPKCS1v15(rand.Reader, jt.privateKey, hash, digest)
		if err != nil {
			jt.t.Fatalf("cannot sign token: %s", err)
		}
		signatureB64 = base64.RawURLEncoding.EncodeToString(signature)
	} else {
		signatureB64 = base64.RawURLEncoding.EncodeToString([]byte("invalid_signature"))
	}

	return payload + "." + signatureB64
}

// setAuthConfig loads cfgStr as the current auth config and returns a function, which restores the previous config.
//
// Usage: defer setAuthConfig(t, cfgStr)()
func setAuthConfig(t testing.TB, cfgStr string) func() {
	t.Helper()

	cfgOrigP := authConfigData.Load()
	if _, err := reloadAuthConfigData([]byte(cfgStr)); err != nil {
		t.Fatalf("cannot load config data: %s", err)
	}
	return func() {
		t.Helper()

		cfgOrig := []byte("unauthorized_user:\n  url_prefix: http://foo/bar")
		if cfgOrigP != nil {
			cfgOrig = *cfgOrigP
		}
		if _, err := reloadAuthConfigData(cfgOrig); err != nil {
			t.Fatalf("cannot restore the original config: %s", err)
		}
	}
}
