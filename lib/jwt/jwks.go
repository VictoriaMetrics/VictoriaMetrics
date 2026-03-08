package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"slices"
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

func ParseJWKs(r io.Reader) (*VerifierPool, error) {
	var resp jwksResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, err
	}

	vs := make([]*verifier, 0, len(resp.Keys))
	for _, key := range resp.Keys {
		if key.Kid == "" {
			return nil, fmt.Errorf("jwks key without kid found")
		}

		switch key.Kty {
		case "RSA":
			if key.E == "" || key.N == "" {
				return nil, fmt.Errorf("jwks key without e or n found")
			}
			e, err := base64.RawURLEncoding.DecodeString(key.E)
			if err != nil {
				return nil, fmt.Errorf("failed to decode jwks key e: %w", err)
			}
			n, err := base64.RawURLEncoding.DecodeString(key.N)
			if err != nil {
				return nil, fmt.Errorf("failed to decode jwks key n: %w", err)
			}

			k := &rsa.PublicKey{
				E: int(big.NewInt(0).SetBytes(e).Int64()),
				N: big.NewInt(0).SetBytes(n),
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
				return nil, fmt.Errorf("jwks key alg %s not allowed; supported %v, %v", key.Alg, rsaAlgs, psAlgs)
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
			if key.Crv == "" || key.X == "" || key.Y == "" {
				return nil, fmt.Errorf("jwks key without crv or x or y found")
			}
			decX, err := base64.RawURLEncoding.DecodeString(key.X)
			if err != nil {
				return nil, fmt.Errorf("failed to decode jwks key x: %w", err)
			}
			decY, err := base64.RawURLEncoding.DecodeString(key.Y)
			if err != nil {
				return nil, fmt.Errorf("failed to decode jwks key y: %w", err)
			}
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
				return nil, fmt.Errorf("jwk %s crv %q unsupported", key.Kty, key.Crv)
			}
			x := big.NewInt(0).SetBytes(decX)
			y := big.NewInt(0).SetBytes(decY)
			if !curve.IsOnCurve(x, y) {
				return nil, fmt.Errorf("jwk %s key invalid; x,y are not on curve %s", key.Kty, key.Crv)
			}

			k := &ecdsa.PublicKey{
				Curve: curve,
				X:     x,
				Y:     y,
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
		}
	}

	return &VerifierPool{
		vs: vs,
	}, nil
}
