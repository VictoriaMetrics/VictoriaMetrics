package jwt

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"fmt"
)

var (
	// ErrSignatureVerificationFailed token signature verification failed
	ErrSignatureVerificationFailed = fmt.Errorf("failed to verify token signature")
	// ErrSignatureAlgorithmNotSupported signature algorithm not supported
	ErrSignatureAlgorithmNotSupported = fmt.Errorf("signature algorithm verification not supported, supported algorithms: RS256, RS384, RS512, PS256, PS384, PS512, ES256, ES384, ES512")
)

var rsaAlgs = []string{"RS256", "RS384", "RS512"}
var psAlgs = []string{"PS256", "PS384", "PS512"}

type verifier struct {
	Verifier

	key any
	alg string
	kid string
}

// VerifierPool is a pool of verifiers for different algorithms
type VerifierPool struct {
	vs []*verifier

	matchKid bool
}

// NewVerifierPool creates a new verifier pool for a set of keys
func NewVerifierPool(keys []any) (*VerifierPool, error) {
	vs := make([]*verifier, 0, len(keys))

	for _, key := range keys {
		switch k := key.(type) {
		case *rsa.PublicKey:
			for _, alg := range rsaAlgs {
				v, err := newVerifierRS(Algorithm(alg), k)
				if err != nil {
					return nil, fmt.Errorf("failed to create RSA verifier for algorithm %s: %w", alg, err)
				}
				vs = append(vs, &verifier{
					Verifier: v,

					key: k,
					alg: alg,
				})
			}

			for _, alg := range psAlgs {
				v, err := newVerifierPS(Algorithm(alg), k)
				if err != nil {
					return nil, fmt.Errorf("failed to create RSA-PSS verifier for algorithm %s: %w", alg, err)
				}
				vs = append(vs, &verifier{
					Verifier: v,

					key: k,
					alg: alg,
				})
			}

		case *ecdsa.PublicKey:
			alg := getAlgorithmForKey(k)
			if alg == "" {
				return nil, fmt.Errorf("failed to create ECDSA verifier: unsupported key")
			}

			v, err := newVerifierES(alg, k)
			if err != nil {
				return nil, fmt.Errorf("failed to create ES verifier for algorithm %s: %w", alg, err)
			}
			vs = append(vs, &verifier{
				Verifier: v,

				key: k,
				alg: string(alg),
			})
		default:
			return nil, fmt.Errorf("unknown key type: %T", key)
		}
	}

	return &VerifierPool{
		vs: vs,
	}, nil
}

// Verify verifies a token signature by using keys provided to verifier pool
func (vp *VerifierPool) Verify(token *Token) error {
	if token.header.Alg == "" {
		return ErrSignatureAlgorithmNotSupported
	}
	for _, v := range vp.vs {
		if token.header.Alg != v.alg {
			continue
		}
		if vp.matchKid && token.header.Kid != "" {
			if token.header.Kid != v.kid {
				continue
			}

			if err := v.Verify(token); err != nil {
				return ErrSignatureVerificationFailed
			}

			return nil
		}

		if err := v.Verify(token); err == nil {
			return nil
		}
	}

	return ErrSignatureAlgorithmNotSupported
}
