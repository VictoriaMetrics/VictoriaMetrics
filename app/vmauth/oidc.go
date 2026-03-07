package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jwt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

type OIDCConfig struct {
	Issuer string `yaml:"issuer"`
}

type oidcDiscovererPool struct {
	ds map[string]*oidcDiscoverer

	context context.Context
	cancel  func()
	wg      *sync.WaitGroup
}

func (dp *oidcDiscovererPool) createOrAdd(issuer string, vp *atomic.Pointer[jwt.VerifierPool]) {
	if dp.ds == nil {
		dp.ds = make(map[string]*oidcDiscoverer)
		dp.context, dp.cancel = context.WithCancel(context.Background())
		dp.wg = &sync.WaitGroup{}
	}

	ds, found := dp.ds[issuer]
	if !found {
		ds = &oidcDiscoverer{
			issuer: issuer,
		}
		dp.ds[issuer] = ds
	}

	ds.vps = append(ds.vps, vp)
}

func (dp *oidcDiscovererPool) startDiscovery() {
	if len(dp.ds) == 0 {
		return
	}

	for _, d := range dp.ds {
		dp.wg.Go(func() {
			if err := d.refreshVerifierPools(dp.context); err != nil {
				logger.Errorf("failed to initialize OIDC verifier pool at start for issuer %q: %s", d.issuer, err)
			}
		})
	}
	dp.wg.Wait()

	for _, d := range dp.ds {
		dp.wg.Go(func() {
			d.run(dp.context)
		})
	}
}

func (dp *oidcDiscovererPool) stopDiscovery() {
	if len(dp.ds) == 0 {
		return
	}

	dp.cancel()
	dp.wg.Wait()
}

type oidcDiscoverer struct {
	issuer string
	vps    []*atomic.Pointer[jwt.VerifierPool]
}

func (d *oidcDiscoverer) run(ctx context.Context) {
	t := time.NewTimer(timeutil.AddJitterToDuration(time.Second * 10))
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if err := d.refreshVerifierPools(ctx); errors.Is(err, context.Canceled) {
				return
			} else if err != nil {
				t.Reset(timeutil.AddJitterToDuration(time.Second * 10))
				logger.Errorf("failed to refresh OIDC verifier pool for issuer %q: %v", d.issuer, err)
				continue
			}
			// OIDC may return Cache-Control header with max-age directive.
			// It could be used as time range for next refresh.
			// https://openid.net/specs/openid-connect-core-1_0.html#RotateEncKeys
			t.Reset(timeutil.AddJitterToDuration(time.Minute * 5))
		case <-ctx.Done():
			return
		}
	}
}

func (d *oidcDiscoverer) refreshVerifierPools(ctx context.Context) error {
	cfg, err := getOpenIDConfiguration(ctx, d.issuer)
	if err != nil {
		return err
	}
	// The issuer in the OIDC configuration must match the expected issuer.
	// https://openid.net/specs/openid-connect-core-1_0.html#RotateEncKeys
	if cfg.Issuer != d.issuer {
		return fmt.Errorf("openid configuration issuer %q does not match expected issuer %q", cfg.Issuer, d.issuer)
	}

	keys, err := fetchJWKs(ctx, cfg.JWKsURI)
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

var oidcHTTPClient = &http.Client{
	Timeout: time.Second * 5,
}

func fetchJWKs(ctx context.Context, jwksURI string) ([]any, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", jwksURI, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for fetching jwks keys from %q: %w", jwksURI, err)
	}

	resp, err := oidcHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch jwks keys from %q: %w", jwksURI, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d when fetching jwks keys from %q", resp.StatusCode, jwksURI)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {

		return nil, fmt.Errorf("failed to decode jwks response from %q: %v", jwksURI, err)
	}

	keys, err := parseJwksKeys(&jwks)
	if err != nil {
		return nil, fmt.Errorf("failed to parse jwks keys from %q: %v", jwksURI, err)
	}

	return keys, nil
}

func getOpenIDConfiguration(ctx context.Context, issuer string) (openidConfig, error) {
	issuer, _ = strings.CutSuffix(issuer, "/")
	configURL := fmt.Sprintf("%s/.well-known/openid-configuration", issuer)

	req, err := http.NewRequestWithContext(ctx, "GET", configURL, http.NoBody)
	if err != nil {
		return openidConfig{}, fmt.Errorf("failed to create request for fetching openid config from %q: %w", configURL, err)
	}

	resp, err := oidcHTTPClient.Do(req)
	if err != nil {
		return openidConfig{}, fmt.Errorf("failed to fetch openid config from %q: %w", configURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return openidConfig{}, fmt.Errorf("unexpected status code %d when fetching openid config from %q", resp.StatusCode, configURL)
	}

	var cfg openidConfig
	if err := json.NewDecoder(resp.Body).Decode(&cfg); err != nil {
		return openidConfig{}, fmt.Errorf("failed to decode openid config from %q: %s", configURL, err)
	}
	_ = resp.Body.Close()

	return cfg, nil
}

func parseJwksKeys(resp *jwksResponse) ([]any, error) {
	keys := make([]any, 0)
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
			exp := big.NewInt(0).SetBytes(e)
			if !exp.IsInt64() || exp.Int64() < 1 {
				return nil, fmt.Errorf("invalid RSA exponent")
			}

			n, err := base64.RawURLEncoding.DecodeString(key.N)
			if err != nil {
				return nil, fmt.Errorf("failed to decode jwks key n: %w", err)
			}
			keys = append(keys, &rsa.PublicKey{
				E: int(exp.Int64()),
				N: big.NewInt(0).SetBytes(n),
			})
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
			keys = append(keys, &ecdsa.PublicKey{
				Curve: curve,
				X:     big.NewInt(0).SetBytes(x),
				Y:     big.NewInt(0).SetBytes(y),
			})
		}
	}

	return keys, nil
}
