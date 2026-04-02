package jwt

import (
	"crypto"
	_ "crypto/sha256" // to register a hash
	_ "crypto/sha512" // to register a hash
	"errors"
)

// Verifier is used to verify tokens.
type Verifier interface {
	Verify(token *Token) error
}

// Algorithm for signing and verifying.
type Algorithm string

func (a Algorithm) String() string { return string(a) }

// Algorithm names for signing and verifying.
const (
	RS256 Algorithm = "RS256"
	RS384 Algorithm = "RS384"
	RS512 Algorithm = "RS512"

	ES256 Algorithm = "ES256"
	ES384 Algorithm = "ES384"
	ES512 Algorithm = "ES512"

	PS256 Algorithm = "PS256"
	PS384 Algorithm = "PS384"
	PS512 Algorithm = "PS512"
)

// JWT sign, verify, build and parse errors.
var (
	// ErrNilKey indicates that key is nil.
	ErrNilKey = errors.New("key is nil")

	// ErrInvalidKey indicates that key is not valid.
	ErrInvalidKey = errors.New("key is not valid")

	// ErrUnsupportedAlg indicates that given algorithm is not supported.
	ErrUnsupportedAlg = errors.New("algorithm is not supported")

	// ErrInvalidSignature indicates that signature is not valid.
	ErrInvalidSignature = errors.New("signature is not valid")
)

func hashPayload(hash crypto.Hash, payload []byte) ([]byte, error) {
	hasher := hash.New()

	if _, err := hasher.Write(payload); err != nil {
		return nil, err
	}
	signed := hasher.Sum(nil)
	return signed, nil
}
