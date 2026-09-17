package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
)

// JWTTester generates RSA key pairs and signed JWT tokens for testing.
type JWTTester struct {
	t            *testing.T
	privateKey   *rsa.PrivateKey
	PublicKeyPEM string
}

// NewJWTTester creates a JWTTester with a freshly generated 2048-bit RSA key pair.
func NewJWTTester(t *testing.T) *JWTTester {
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

	return &JWTTester{
		t:            t,
		privateKey:   privateKey,
		PublicKeyPEM: publicKeyPEM,
	}
}

// NewVerifierPool creates a VerifierPool from the test RSA public key.
func (jt *JWTTester) NewVerifierPool() *VerifierPool {
	jt.t.Helper()
	vp, err := NewVerifierPool([]any{&jt.privateKey.PublicKey})
	if err != nil {
		jt.t.Fatalf("cannot create verifier pool: %s", err)
	}
	return vp
}

// GenToken generates a signed JWT with the given body claims.
// If valid is false, the signature is invalid.
func (jt *JWTTester) GenToken(body map[string]any, valid bool) string {
	jt.t.Helper()

	headerJSON, err := json.Marshal(map[string]any{
		"alg": "RS256",
		"typ": "JWT",
	})
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
