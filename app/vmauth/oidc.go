package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/jwt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

type oidcConfig struct {
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

	verifierPool, err := fetchAndParseJWKs(ctx, cfg.JWKsURI)
	if err != nil {
		return err
	}

	for _, vp := range d.vps {
		vp.Store(verifierPool)
	}
	return nil
}

// See https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata for details.
type openidConfig struct {
	Issuer  string `json:"issuer"`
	JWKsURI string `json:"jwks_uri"`
}

var oidcHTTPClient = &http.Client{
	Timeout: time.Second * 5,
}

func fetchAndParseJWKs(ctx context.Context, jwksURI string) (*jwt.VerifierPool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURI, nil)
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

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %q: %w", jwksURI, err)
	}

	vp, err := jwt.ParseJWKs(b)
	if err != nil {
		return nil, fmt.Errorf("failed to parse jwks keys from %q: %v", jwksURI, err)
	}

	return vp, nil
}

func getOpenIDConfiguration(ctx context.Context, issuer string) (openidConfig, error) {
	issuer, _ = strings.CutSuffix(issuer, "/")
	configURL := fmt.Sprintf("%s/.well-known/openid-configuration", issuer)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
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

	return cfg, nil
}
