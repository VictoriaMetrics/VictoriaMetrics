package jwt

import (
	"crypto"
	"crypto/rsa"
)

// newVerifierRS returns a new RSA-based verifier.
func newVerifierRS(alg Algorithm, key *rsa.PublicKey) (*rsAlg, error) {
	if key == nil {
		return nil, ErrNilKey
	}
	hash, err := getHashRS(alg)
	if err != nil {
		return nil, err
	}
	return &rsAlg{
		alg:       alg,
		hash:      hash,
		publicKey: key,
	}, nil
}

func getHashRS(alg Algorithm) (crypto.Hash, error) {
	var hash crypto.Hash
	switch alg {
	case RS256:
		hash = crypto.SHA256
	case RS384:
		hash = crypto.SHA384
	case RS512:
		hash = crypto.SHA512
	default:
		return 0, ErrUnsupportedAlg
	}
	return hash, nil
}

type rsAlg struct {
	alg       Algorithm
	hash      crypto.Hash
	publicKey *rsa.PublicKey
}

func (rs *rsAlg) Verify(token *Token) error {
	return rs.verify(token.payload, token.signature)
}

func (rs *rsAlg) verify(payload, signature []byte) error {
	digest, err := hashPayload(rs.hash, payload)
	if err != nil {
		return err
	}

	errVerify := rsa.VerifyPKCS1v15(rs.publicKey, rs.hash, digest, signature)
	if errVerify != nil {
		return ErrInvalidSignature
	}
	return nil
}
