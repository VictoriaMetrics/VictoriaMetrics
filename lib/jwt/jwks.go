package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// See https://www.rfc-editor.org/rfc/rfc7517 for details.
type jwk struct {
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Kid string `json:"kid"`

	// RSA keys contents
	E string `json:"e"`
	N string `json:"n"`

	// EC keys contents
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

// ParseJWKs parses a JSON Web Key Set (JWKS) from rawResp and returns a VerifierPool
// containing a verifier for each key in the set. Each key might have a non-empty "kid" field.
// For RSA keys, if "alg" is specified it must be one of the supported RS or PS algorithms;
// if omitted, verifiers are created for all supported RSA and RSA-PSS algorithms.
// For EC keys, the curve determines the algorithm. It must match "alg" if provided.
//
// The returned VerifierPool matches tokens by "kid" if not empty, otherwise tries all keys.
func ParseJWKs(rawResp []byte) (*VerifierPool, error) {
	var resp jwksResponse
	if err := json.Unmarshal(rawResp, &resp); err != nil {
		return nil, err
	}

	vs := make([]*verifier, 0, len(resp.Keys))
	for _, key := range resp.Keys {
		// Skip non-signature keys
		// see https://datatracker.ietf.org/doc/html/rfc7517#section-4.2
		if key.Use != "" && key.Use != "sig" {
			continue
		}
		switch key.Kty {
		case "RSA":
			k, err := parseRSAPublicKey(&key)
			if err != nil {
				logger.Warnf("skipping JWKS RSA key kid=%s: %s", key.Kid, err)
				continue
			}

			if slices.Contains(rsaAlgs, key.Alg) {
				v, err := newVerifierRS(Algorithm(key.Alg), k)
				if err != nil {
					return nil, fmt.Errorf("failed to create RSA verifier for algorithm %s: %w", key.Alg, err)
				}
				vs = append(vs, &verifier{
					Verifier: v,

					key: k,
					alg: key.Alg,
					kid: key.Kid,
				})

				continue
			}

			if slices.Contains(psAlgs, key.Alg) {
				v, err := newVerifierPS(Algorithm(key.Alg), k)
				if err != nil {
					return nil, fmt.Errorf("failed to create RSA-PSS verifier for algorithm %s: %w", key.Alg, err)
				}
				vs = append(vs, &verifier{
					Verifier: v,

					key: k,
					alg: key.Alg,
					kid: key.Kid,
				})

				continue
			}

			if key.Alg != "" {
				if key.Use == "sig" {
					return nil, fmt.Errorf("jwks key with use=sig has unsupported alg %s; supported %v, %v", key.Alg, rsaAlgs, psAlgs)
				}
				logger.Warnf("skipping JWKS RSA key kid=%s with unsupported alg=%s", key.Kid, key.Alg)
				continue
			}

			for _, alg := range rsaAlgs {
				v, err := newVerifierRS(Algorithm(alg), k)
				if err != nil {
					return nil, fmt.Errorf("failed to create RSA verifier for algorithm %s: %w", alg, err)
				}
				vs = append(vs, &verifier{
					Verifier: v,

					key: k,
					alg: alg,
					kid: key.Kid,
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
					kid: key.Kid,
				})
			}
		case "EC":
			k, alg, err := parseECPublicKey(&key)
			if err != nil {
				logger.Warnf("skipping JWKS EC key kid=%s: %s", key.Kid, err)
				continue
			}

			if key.Alg != "" && key.Alg != string(alg) {
				if key.Use == "sig" {
					return nil, fmt.Errorf("jwk with use=sig has alg %s that does not match curve %s", key.Alg, key.Crv)
				}
				logger.Warnf("skipping JWKS EC key kid=%s: alg=%s does not match curve=%s", key.Kid, key.Alg, key.Crv)
				continue
			}

			v, err := newVerifierES(alg, k)
			if err != nil {
				return nil, fmt.Errorf("failed to create ES verifier for algorithm %s: %w", alg, err)
			}
			vs = append(vs, &verifier{
				Verifier: v,

				key: k,
				alg: string(alg),
				kid: key.Kid,
			})
		default:
			// Unsupported key types should be ignored, see https://www.rfc-editor.org/rfc/rfc7517#section-5
			logger.Warnf("skipping JWKS key kid=%s with unsupported kty=%s; supported: RSA, EC", key.Kid, key.Kty)
		}
	}

	return &VerifierPool{
		matchKid: true,
		vs:       vs,
	}, nil
}

// parseRSAPublicKey and parseECPublicKey return an error for keys that must be skipped:
// keys with unsupported parameters, missing required members or invalid values.
// See https://www.rfc-editor.org/rfc/rfc7517#section-5
func parseRSAPublicKey(key *jwk) (*rsa.PublicKey, error) {
	if key.E == "" || key.N == "" {
		return nil, fmt.Errorf("missing e or n")
	}
	e, err := base64.RawURLEncoding.DecodeString(key.E)
	if err != nil {
		return nil, fmt.Errorf("cannot decode e: %w", err)
	}
	exp := big.NewInt(0).SetBytes(e)
	if !exp.IsInt64() || exp.Int64() < 1 {
		return nil, fmt.Errorf("invalid exponent")
	}
	n, err := base64.RawURLEncoding.DecodeString(key.N)
	if err != nil {
		return nil, fmt.Errorf("cannot decode n: %w", err)
	}
	return &rsa.PublicKey{
		E: int(exp.Int64()),
		N: big.NewInt(0).SetBytes(n),
	}, nil
}

func parseECPublicKey(key *jwk) (*ecdsa.PublicKey, Algorithm, error) {
	var curve elliptic.Curve
	var alg Algorithm
	switch key.Crv {
	case "P-256":
		curve = elliptic.P256()
		alg = ES256
	case "P-384":
		curve = elliptic.P384()
		alg = ES384
	case "P-521":
		curve = elliptic.P521()
		alg = ES512
	default:
		return nil, "", fmt.Errorf("unsupported crv=%q; supported: P-256, P-384, P-521", key.Crv)
	}
	if key.X == "" || key.Y == "" {
		return nil, "", fmt.Errorf("missing x or y")
	}
	decX, err := base64.RawURLEncoding.DecodeString(key.X)
	if err != nil {
		return nil, "", fmt.Errorf("cannot decode x: %w", err)
	}
	decY, err := base64.RawURLEncoding.DecodeString(key.Y)
	if err != nil {
		return nil, "", fmt.Errorf("cannot decode y: %w", err)
	}
	x := big.NewInt(0).SetBytes(decX)
	y := big.NewInt(0).SetBytes(decY)
	if !curve.IsOnCurve(x, y) {
		return nil, "", fmt.Errorf("x,y are not on curve %s", key.Crv)
	}
	return &ecdsa.PublicKey{
		Curve: curve,
		X:     x,
		Y:     y,
	}, alg, nil
}
