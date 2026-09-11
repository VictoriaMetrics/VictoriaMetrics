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

// subscribeToMetadata registers pm to receive OIDC provider metadata for the given issuer.
// All subscribe calls must be done before startDiscovery.
// startDiscovery performs the first discovery synchronously; if it fails, pm stays nil until the next periodic refresh.
// Callers must check the atomic value for nil before use.
func (dp *oidcDiscovererPool) subscribeToMetadata(issuer string, pm *atomic.Pointer[oidcProviderMetadata]) {
	if pm == nil {
		logger.Panicf("BUG: pm pointer is nil")
	}
	ds := dp.getDiscoverer(issuer)
	ds.pms = append(ds.pms, pm)
}

// subscribeToVerifier registers vp to receive OIDC jwt verifier for the given issuer.
// All subscribe calls must be done before startDiscovery.
// startDiscovery performs the first discovery synchronously; if it fails, vp stays nil until the next periodic refresh.
// Callers must check the atomic value for nil before use.
func (dp *oidcDiscovererPool) subscribeToVerifier(issuer string, vp *atomic.Pointer[jwt.VerifierPool]) {
	if vp == nil {
		logger.Panicf("BUG: vp pointer is nil")
	}
	ds := dp.getDiscoverer(issuer)
	ds.vps = append(ds.vps, vp)
}

// getDiscoverer returns the oidcDiscoverer for issuer, creating it if needed.
// It also initializes the pool on first call.
func (dp *oidcDiscovererPool) getDiscoverer(issuer string) *oidcDiscoverer {
	if dp.ds == nil {
		dp.ds = make(map[string]*oidcDiscoverer)
		dp.context, dp.cancel = context.WithCancel(context.Background())
		dp.wg = &sync.WaitGroup{}
	}
	ds, found := dp.ds[issuer]
	if !found {
		ds = &oidcDiscoverer{issuer: issuer}
		dp.ds[issuer] = ds
	}
	return ds
}

func (dp *oidcDiscovererPool) startDiscovery() {
	if len(dp.ds) == 0 {
		return
	}

	for _, d := range dp.ds {
		dp.wg.Go(func() {
			if err := d.refreshMetadata(dp.context); err != nil {
				logger.Errorf("failed to refresh OIDC provider metadata at start for issuer %q: %s", d.issuer, err)
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
	pms    []*atomic.Pointer[oidcProviderMetadata]
	vps    []*atomic.Pointer[jwt.VerifierPool]
}

func (d *oidcDiscoverer) run(ctx context.Context) {
	t := time.NewTimer(timeutil.AddJitterToDuration(time.Second * 10))
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if err := d.refreshMetadata(ctx); errors.Is(err, context.Canceled) {
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

func (d *oidcDiscoverer) refreshMetadata(ctx context.Context) error {
	newPM, err := getOIDCProviderMetadata(ctx, d.issuer)
	if err != nil {
		return err
	}
	// The issuer in the OIDC configuration must match the expected issuer.
	// https://openid.net/specs/openid-connect-core-1_0.html#RotateEncKeys
	if newPM.Issuer != d.issuer {
		return fmt.Errorf("openid configuration issuer %q does not match expected issuer %q", newPM.Issuer, d.issuer)
	}
	newVP, err := fetchAndParseJWKs(ctx, newPM.JWKsURI)
	if err != nil {
		return err
	}
	newPM.vp = newVP

	for _, pm := range d.pms {
		pm.Store(&newPM)
	}
	for _, vp := range d.vps {
		vp.Store(newVP)
	}
	return nil
}

// See https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata for details.
type oidcProviderMetadata struct {
	Issuer                string `json:"issuer"`
	JWKsURI               string `json:"jwks_uri"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`

	vp *jwt.VerifierPool
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

	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body from %q: %w", jwksURI, err)
	}

	vp, err := jwt.ParseJWKs(b)
	if err != nil {
		return nil, fmt.Errorf("failed to parse jwks keys from %q: %w", jwksURI, err)
	}

	return vp, nil
}

func getOIDCProviderMetadata(ctx context.Context, issuer string) (oidcProviderMetadata, error) {
	issuer, _ = strings.CutSuffix(issuer, "/")
	configURL := fmt.Sprintf("%s/.well-known/openid-configuration", issuer)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configURL, nil)
	if err != nil {
		return oidcProviderMetadata{}, fmt.Errorf("failed to create request for fetching openid config from %q: %w", configURL, err)
	}

	resp, err := oidcHTTPClient.Do(req)
	if err != nil {
		return oidcProviderMetadata{}, fmt.Errorf("failed to fetch openid config from %q: %w", configURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return oidcProviderMetadata{}, fmt.Errorf("unexpected status code %d when fetching openid config from %q", resp.StatusCode, configURL)
	}

	var pm oidcProviderMetadata
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&pm); err != nil {
		return oidcProviderMetadata{}, fmt.Errorf("failed to decode openid config from %q: %w", configURL, err)
	}

	return pm, nil
}
