package jwt

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// ParseKey parses key in PEM format.
// It returns a *rsa.PublicKey, *dsa.PublicKey, *ecdsa.PublicKey, or ed25519.PublicKey.
func ParseKey(key []byte) (any, error) {
	b, _ := pem.Decode(key)
	if b == nil {
		return nil, fmt.Errorf("failed to parse key %q: failed to decode PEM block containing public key", key)
	}

	k, err := x509.ParsePKIXPublicKey(b.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse key %q: %v", key, err)
	}

	return k, nil
}
