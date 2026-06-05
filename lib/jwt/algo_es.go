package jwt

import (
	"crypto"
	"crypto/ecdsa"
	"math/big"
)

// newVerifierES returns a new ECDSA-based verifier.
func newVerifierES(alg Algorithm, key *ecdsa.PublicKey) (*esAlg, error) {
	if key == nil {
		return nil, ErrNilKey
	}
	hash, err := getParamsES(alg, roundBytes(key.Params().BitSize)*2)
	if err != nil {
		return nil, err
	}
	return &esAlg{
		alg:       alg,
		hash:      hash,
		publicKey: key,
		signSize:  signSize(key),
	}, nil
}

func signSize(key *ecdsa.PublicKey) int {
	return roundBytes(key.Params().BitSize) * 2
}

func getAlgorithmForKey(key *ecdsa.PublicKey) Algorithm {
	switch signSize(key) {
	case 64:
		return ES256
	case 96:
		return ES384
	case 132:
		return ES512
	}
	return ""
}

func getParamsES(alg Algorithm, size int) (crypto.Hash, error) {
	var hash crypto.Hash
	var keySize int

	switch alg {
	case ES256:
		hash, keySize = crypto.SHA256, 64
	case ES384:
		hash, keySize = crypto.SHA384, 96
	case ES512:
		hash, keySize = crypto.SHA512, 132
	default:
		return 0, ErrUnsupportedAlg
	}

	if keySize != size {
		return 0, ErrInvalidKey
	}
	return hash, nil
}

type esAlg struct {
	alg       Algorithm
	hash      crypto.Hash
	publicKey *ecdsa.PublicKey
	signSize  int
}

func (es *esAlg) SignSize() int {
	return es.signSize
}

func (es *esAlg) Verify(token *Token) error {
	return es.verify(token.payload, token.signature)
}

func (es *esAlg) verify(payload, signature []byte) error {
	if len(signature) != es.SignSize() {
		return ErrInvalidSignature
	}

	digest, err := hashPayload(es.hash, payload)
	if err != nil {
		return err
	}

	pivot := es.SignSize() / 2
	r := big.NewInt(0).SetBytes(signature[:pivot])
	s := big.NewInt(0).SetBytes(signature[pivot:])

	if !ecdsa.Verify(es.publicKey, digest, r, s) {
		return ErrInvalidSignature
	}
	return nil
}

func roundBytes(n int) int {
	res := n / 8
	if n%8 > 0 {
		return res + 1
	}
	return res
}
