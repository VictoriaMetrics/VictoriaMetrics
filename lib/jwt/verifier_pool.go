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

// VerifierPool is a pool of verifiers for different algorithms
type VerifierPool struct {
	rsaVerifiers map[string][]Verifier
	psVerifiers  map[string][]Verifier
	esVerifiers  map[string][]Verifier
}

// NewVerifierPool creates a new verifier pool for a set of keys
func NewVerifierPool(keys []any) (*VerifierPool, error) {
	rsaVerifiers := make(map[string][]Verifier)
	psVerifiers := make(map[string][]Verifier)
	esVerifiers := make(map[string][]Verifier)

	rsaAlgs := []string{"RS256", "RS384", "RS512"}
	psAlgs := []string{"PS256", "PS384", "PS512"}

	for _, key := range keys {
		switch k := key.(type) {
		case *rsa.PublicKey:
			for _, alg := range rsaAlgs {
				verifier, err := newVerifierRS(Algorithm(alg), k)
				if err != nil {
					return nil, fmt.Errorf("failed to create RSA verifier for algorithm %s: %w", alg, err)
				}
				rsaVerifiers[alg] = append(rsaVerifiers[alg], verifier)
			}

			for _, alg := range psAlgs {
				verifier, err := newVerifierPS(Algorithm(alg), k)
				if err != nil {
					return nil, fmt.Errorf("failed to create RSA-PSS verifier for algorithm %s: %w", alg, err)
				}
				psVerifiers[alg] = append(psVerifiers[alg], verifier)
			}

		case *ecdsa.PublicKey:
			alg := getAlgorithmForKey(k)
			if alg == "" {
				return nil, fmt.Errorf("failed to create ECDSA verifier: unsupported key")
			}

			verifier, err := newVerifierES(alg, k)
			if err != nil {
				return nil, fmt.Errorf("failed to create ES verifier for algorithm %s: %w", alg, err)
			}
			esVerifiers[string(alg)] = append(esVerifiers[string(alg)], verifier)

		default:
			return nil, fmt.Errorf("unknown key type: %T", key)
		}
	}

	return &VerifierPool{
		rsaVerifiers: rsaVerifiers,
		psVerifiers:  psVerifiers,
		esVerifiers:  esVerifiers,
	}, nil
}

func (vp *VerifierPool) getVerifiers(alg string) []Verifier {
	if len(alg) < 2 {
		return nil
	}

	switch alg[:2] {
	case "RS":
		return vp.getRSVerifiers(alg)
	case "PS":
		return vp.getPSVerifiers(alg)
	case "ES":
		return vp.getESVerifiers(alg)
	default:
		return nil
	}
}

// Verify verifies a token signature by using keys provided to verifier pool
func (vp *VerifierPool) Verify(token *Token) error {
	verifiers := vp.getVerifiers(token.header.Alg)
	if verifiers == nil {
		return ErrSignatureAlgorithmNotSupported
	}

	for _, verifier := range verifiers {
		err := verifier.Verify(token)
		if err == nil {
			// Token verified, returning success immediately
			return nil
		}
	}

	return ErrSignatureVerificationFailed
}

func (vp *VerifierPool) getRSVerifiers(alg string) []Verifier {
	v, ok := vp.rsaVerifiers[alg]
	if ok {
		return v
	}

	return nil
}

func (vp *VerifierPool) getPSVerifiers(alg string) []Verifier {
	v, ok := vp.psVerifiers[alg]
	if ok {
		return v
	}

	return nil
}

func (vp *VerifierPool) getESVerifiers(alg string) []Verifier {
	v, ok := vp.esVerifiers[alg]
	if ok {
		return v
	}

	return nil
}
