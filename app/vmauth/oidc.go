package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jwt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type OIDCConfig struct {
	Issuer string `yaml:"issuer"`
}

type oidcDiscoverers struct {
	ds      map[string]*oidcDiscoverer
	context context.Context
	cancel  func()
	wg      *sync.WaitGroup
}

func (ds *oidcDiscoverers) stop() {
	ds.cancel()
	ds.wg.Wait()
}

func (ds *oidcDiscoverers) add(iss string, vp *atomic.Pointer[jwt.VerifierPool]) {
	d, ok := ds.ds[iss]
	if !ok {
		d = &oidcDiscoverer{
			issuer:  iss,
			context: ds.context,
			cancel:  ds.cancel,
			wg:      ds.wg,
		}
		ds.ds[iss] = d
	}

	d.vps = append(d.vps, vp)
}

func (ds *oidcDiscoverers) start() {
	for _, d := range ds.ds {
		d.start()
	}
}

type oidcDiscoverer struct {
	vps    []*atomic.Pointer[jwt.VerifierPool]
	issuer string

	context context.Context
	cancel  func()
	wg      *sync.WaitGroup
}

func (d *oidcDiscoverer) start() {
	if err := d.refreshVerifierPool(); err != nil {
		logger.Errorf("failed to refresh OIDC verifier pool for issuer %q: %v", d.issuer, err)
	}

	d.wg.Go(func() {
		t := time.NewTimer(time.Second * 10)
		defer t.Stop()

		for {
			select {
			case <-t.C:
				if err := d.refreshVerifierPool(); err != nil {
					t.Reset(time.Second * 10)
					logger.Errorf("failed to refresh OIDC verifier pool for issuer %q: %v", d.issuer, err)
				}
				// OIDC may reutrn Cache-Control header with max-age directive.
				// It could be used as time rage for next refresh.
				// https://openid.net/specs/openid-connect-core-1_0.html#RotateEncKeys
				t.Reset(time.Minute * 5)
			case <-d.context.Done():
				return
			}
		}
	})
}

func (d *oidcDiscoverer) refreshVerifierPool() error {
	cfg, err := getOpenIDConfiguration(d.issuer)
	if err != nil {
		return err
	}
	// The issuer in the OIDC configuration must match the expected issuer.
	// https://openid.net/specs/openid-connect-core-1_0.html#RotateEncKeys
	if cfg.Issuer != d.issuer {
		return fmt.Errorf("openid configuration issuer %q does not match expected issuer %q", cfg.Issuer, d.issuer)
	}

	keys, err := fetchJWKs(cfg.JWKsURI)
	if err != nil {
		return err
	}

	verifierPool, err := jwt.NewVerifierPool(keys)
	if err != nil {
		return err
	}

	for _, vp := range d.vps {
		vp.Store(verifierPool)
	}

	return nil
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

// See https://www.rfc-editor.org/rfc/rfc7517 for details.
type jwk struct {
	Type string `json:"kty"`
	Alg  string `json:"alg"`
	Use  string `json:"use"`
	Kid  string `json:"kid"`

	// RSA keys contents
	E string `json:"e"`
	N string `json:"n"`

	// EC keys contents
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

// See https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata for details.
type openidConfig struct {
	Issuer  string `json:"issuer"`
	JWKsURI string `json:"jwks_uri"`
}

func fetchJWKs(jwksURI string) ([]any, error) {
	resp, err := http.Get(jwksURI)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch jwks keys from %q: %v", jwksURI, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d when fetching jwks keys from %q", resp.StatusCode, jwksURI)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {

		return nil, fmt.Errorf("failed to decode jwks response from %q: %v", jwksURI, err)
	}
	_ = resp.Body.Close()

	keys, err := parseJwksKeys(&jwks)
	if err != nil {
		return nil, fmt.Errorf("failed to parse jwks keys from %q: %v", jwksURI, err)
	}

	return keys, nil
}

func getOpenIDConfiguration(issuer string) (openidConfig, error) {
	issuer, _ = strings.CutSuffix(issuer, "/")
	configURL := fmt.Sprintf("%s/.well-known/openid-configuration", issuer)

	resp, err := http.Get(configURL)
	if err != nil {
		return openidConfig{}, fmt.Errorf("failed to fetch openid config from %q: %v", configURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return openidConfig{}, fmt.Errorf("unexpected status code %d when fetching openid config from %q", resp.StatusCode, configURL)
	}

	var cfg openidConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return openidConfig{}, fmt.Errorf("failed to decode openid config from %q: %v", configURL, err)
	}
	_ = resp.Body.Close()

	return cfg, nil
}

func parseJwksKeys(resp *jwksResponse) ([]any, error) {
	keys := make(map[string]any)
	for _, key := range resp.Keys {
		if key.Kid == "" {
			return nil, fmt.Errorf("jwks key without kid found")
		}

		switch key.Type {
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
			keys[key.Kid] = &rsa.PublicKey{
				E: int(big.NewInt(0).SetBytes(e).Int64()),
				N: big.NewInt(0).SetBytes(n),
			}
		case "EC":
			if key.Crv == "" || key.X == "" || key.Y == "" {
				return nil, fmt.Errorf("jwks key without crv or x or y found")
			}
			x, err := base64.RawURLEncoding.DecodeString(key.X)
			if err != nil {
				return nil, fmt.Errorf("failed to decode jwks key x: %w", err)
			}
			y, err := base64.RawURLEncoding.DecodeString(key.Y)
			if err != nil {
				return nil, fmt.Errorf("failed to decode jwks key y: %w", err)
			}
			var curve elliptic.Curve
			switch key.Crv {
			case "P-256":
				curve = elliptic.P256()
			case "P-384":
				curve = elliptic.P384()
			case "P-521":
				curve = elliptic.P521()
			default:
				return nil, fmt.Errorf("unsupported jwks key crv %q found", key.Crv)
			}
			keys[key.Kid] = &ecdsa.PublicKey{
				Curve: curve,
				X:     big.NewInt(0).SetBytes(x),
				Y:     big.NewInt(0).SetBytes(y),
			}
		}
	}

	keysValues := make([]any, 0)
	for _, key := range keys {
		keysValues = append(keysValues, key)
	}

	return keysValues, nil
}
