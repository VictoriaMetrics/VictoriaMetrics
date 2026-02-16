package jwt

import (
	"crypto"
	"crypto/rsa"
)

// newVerifierPS returns a new RSA-PSS-based verifier.
func newVerifierPS(alg Algorithm, key *rsa.PublicKey) (*psAlg, error) {
	if key == nil {
		return nil, ErrNilKey
	}
	hash, opts, err := getParamsPS(alg)
	if err != nil {
		return nil, err
	}
	return &psAlg{
		alg:       alg,
		hash:      hash,
		publicKey: key,
		opts:      opts,
	}, nil
}

func getParamsPS(alg Algorithm) (crypto.Hash, *rsa.PSSOptions, error) {
	switch alg {
	case PS256:
		return crypto.SHA256, optsPS256, nil
	case PS384:
		return crypto.SHA384, optsPS384, nil
	case PS512:
		return crypto.SHA512, optsPS512, nil
	default:
		return 0, nil, ErrUnsupportedAlg
	}
}

var (
	optsPS256 = &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthAuto,
		Hash:       crypto.SHA256,
	}

	optsPS384 = &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthAuto,
		Hash:       crypto.SHA384,
	}

	optsPS512 = &rsa.PSSOptions{
		SaltLength: rsa.PSSSaltLengthAuto,
		Hash:       crypto.SHA512,
	}
)

type psAlg struct {
	alg       Algorithm
	hash      crypto.Hash
	publicKey *rsa.PublicKey
	opts      *rsa.PSSOptions
}

func (ps *psAlg) Verify(token *Token) error {
	return ps.verify(token.payload, token.signature)
}

func (ps *psAlg) verify(payload, signature []byte) error {
	digest, err := hashPayload(ps.hash, payload)
	if err != nil {
		return err
	}

	errVerify := rsa.VerifyPSS(ps.publicKey, ps.hash, digest, signature, ps.opts)
	if errVerify != nil {
		return ErrInvalidSignature
	}
	return nil
}
