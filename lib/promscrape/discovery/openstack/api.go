package openstack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

var configMap = discoveryutil.NewConfigMap()

// apiCredentials can be refreshed
type apiCredentials struct {
	computeURL *url.URL
	token      string
	expiration time.Time
}

type apiConfig struct {
	client *http.Client
	port   int
	// tokenLock guards creds refresh
	tokenLock sync.Mutex
	creds     *apiCredentials
	// authTokenReq contains request body for apiCredentials
	authTokenReq []byte
	// keystone endpoint
	endpoint   *url.URL
	allTenants bool
	region     string
	// availability public, internal, admin for filtering compute endpoint
	availability string
}

func (cfg *apiConfig) getFreshAPICredentials() (*apiCredentials, error) {
	cfg.tokenLock.Lock()
	defer cfg.tokenLock.Unlock()

	if cfg.creds != nil && time.Until(cfg.creds.expiration) > 10*time.Second {
		// Credentials aren't expired yet.
		return cfg.creds, nil
	}
	newCreds, err := getCreds(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot refresh OpenStack api token: %w", err)
	}
	cfg.creds = newCreds
	logger.Infof("successfully refreshed OpenStack api token; expiration: %.3f seconds", time.Until(newCreds.expiration).Seconds())
	return newCreds, nil
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	port := sdc.Port
	if port == 0 {
		port = 80
	}

	tr := httputil.NewTransport(false, "vm_promscrape_discovery_openstack")
	tr.MaxIdleConnsPerHost = 100

	cfg := &apiConfig{
		client: &http.Client{
			Transport: tr,
		},
		availability: sdc.Availability,
		region:       sdc.Region,
		allTenants:   sdc.AllTenants,
		port:         port,
	}
	if sdc.TLSConfig != nil {
		opts := &promauth.Options{
			BaseDir:   baseDir,
			TLSConfig: sdc.TLSConfig,
		}
		ac, err := opts.NewConfig()
		if err != nil {
			cfg.client.CloseIdleConnections()
			return nil, fmt.Errorf("cannot parse TLS config: %w", err)
		}
		cfg.client.Transport = ac.NewRoundTripper(tr)
	}
	// use public compute endpoint by default
	if len(cfg.availability) == 0 {
		cfg.availability = "public"
	}

	// create new variable to prevent side effects
	sdcAuth := *sdc
	// special case if identity_endpoint is not defined
	if len(sdcAuth.IdentityEndpoint) == 0 {
		// override sdc
		sdcAuth = readCredentialsFromEnv()
	}
	if strings.HasSuffix(sdcAuth.IdentityEndpoint, "v2.0") {
		cfg.client.CloseIdleConnections()
		return nil, errors.New("identity_endpoint v2.0 is not supported")
	}
	// trim .0 from v3.0 for prometheus cfg compatibility
	sdcAuth.IdentityEndpoint = strings.TrimSuffix(sdcAuth.IdentityEndpoint, ".0")

	parsedURL, err := url.Parse(sdcAuth.IdentityEndpoint)
	if err != nil {
		cfg.client.CloseIdleConnections()
		return nil, fmt.Errorf("cannot parse identity_endpoint %s as url: %w", sdcAuth.IdentityEndpoint, err)
	}
	cfg.endpoint = parsedURL
	tokenReq, err := buildAuthRequestBody(&sdcAuth)
	if err != nil {
		cfg.client.CloseIdleConnections()
		return nil, err
	}
	cfg.authTokenReq = tokenReq
	// cfg.creds is populated at getFreshAPICredentials

	return cfg, nil
}

// getCreds makes a call to openstack keystone api and retrieves token and computeURL
//
// See https://docs.openstack.org/api-ref/identity/v3/
func getCreds(cfg *apiConfig) (*apiCredentials, error) {
	apiURL := *cfg.endpoint
	apiURL.Path = path.Join(apiURL.Path, "auth", "tokens")

	resp, err := cfg.client.Post(apiURL.String(), "application/json", bytes.NewBuffer(cfg.authTokenReq))
	if err != nil {
		return nil, fmt.Errorf("failed query openstack identity api at url %s: %w", apiURL.String(), err)
	}
	r, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot read response from %q: %w", apiURL.String(), err)
	}
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("auth failed, bad status code: %d, want: 201", resp.StatusCode)
	}
	at := resp.Header.Get("X-Subject-Token")
	if len(at) == 0 {
		return nil, fmt.Errorf("auth failed, response without X-Subject-Token")
	}
	var ar authResponse
	if err := json.Unmarshal(r, &ar); err != nil {
		return nil, fmt.Errorf("cannot parse auth credentials response: %w", err)
	}
	computeURL, err := getComputeEndpointURL(ar.Token.Catalog, cfg.availability, cfg.region)
	if err != nil {
		return nil, fmt.Errorf("cannot get computeEndpoint, account doesn't have enough permissions, "+
			"availability: %s, region: %s; error: %w", cfg.availability, cfg.region, err)
	}
	return &apiCredentials{
		token:      at,
		expiration: ar.Token.ExpiresAt,
		computeURL: computeURL,
	}, nil
}

// readResponseBody reads body from http.Response.
func readResponseBody(resp *http.Response, apiURL string) ([]byte, error) {
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot read response from %q: %w", apiURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code for %q; got %d; want %d; response body: %q",
			apiURL, resp.StatusCode, http.StatusOK, data)
	}
	return data, nil
}

// getAPIResponse calls openstack apiURL and returns response body.
func getAPIResponse(apiURL string, cfg *apiConfig) ([]byte, error) {
	creds, err := cfg.getFreshAPICredentials()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create new request for openstack api url %s: %w", apiURL, err)
	}
	req.Header.Set("X-Auth-Token", creds.token)
	resp, err := cfg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot query openstack api url %s: %w", apiURL, err)
	}
	return readResponseBody(resp, apiURL)

}
