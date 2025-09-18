package yandexcloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

var configMap = discoveryutil.NewConfigMap()

type apiCredentials struct {
	Token      string    `json:"Token"`
	Expiration time.Time `json:"Expiration"`
}

// yandexPassportOAuth is a struct for Yandex Cloud IAM token request
// https://cloud.yandex.com/en-ru/docs/iam/operations/iam-token/create
type yandexPassportOAuth struct {
	YandexPassportOAuthToken string `json:"yandexPassportOauthToken"`
}

type apiConfig struct {
	client              *http.Client
	yandexPassportOAuth *yandexPassportOAuth
	serviceEndpoints    map[string]string

	// credsLock protects the refresh of creds
	credsLock sync.Mutex
	creds     *apiCredentials
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	tr := httputil.NewTransport(false, "vm_promscrape_discovery_yandex")
	tr.MaxIdleConnsPerHost = 100
	rt := http.RoundTripper(tr)

	if sdc.TLSConfig != nil {
		opts := &promauth.Options{
			BaseDir:   baseDir,
			TLSConfig: sdc.TLSConfig,
		}
		ac, err := opts.NewConfig()
		if err != nil {
			return nil, fmt.Errorf("cannot parse TLS config: %w", err)
		}
		rt = ac.NewRoundTripper(tr)
	}
	cfg := &apiConfig{
		client: &http.Client{
			Transport: rt,
		},
	}
	apiEndpoint := sdc.APIEndpoint
	if apiEndpoint == "" {
		apiEndpoint = "https://api.cloud.yandex.net"
	}
	serviceEndpoints, err := cfg.getServiceEndpoints(apiEndpoint)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain endpoints for yandex services: %w", err)
	}
	cfg.serviceEndpoints = serviceEndpoints
	if sdc.YandexPassportOAuthToken != nil {
		logger.Infof("yandexcloud_sd: using yandex passport OAuth token")
		cfg.yandexPassportOAuth = &yandexPassportOAuth{
			YandexPassportOAuthToken: sdc.YandexPassportOAuthToken.String(),
		}
	}
	return cfg, nil
}

// getFreshAPICredentials checks token lifetime and update if needed
func (cfg *apiConfig) getFreshAPICredentials() (*apiCredentials, error) {
	cfg.credsLock.Lock()
	defer cfg.credsLock.Unlock()

	if cfg.creds != nil && time.Until(cfg.creds.Expiration) > 10*time.Second {
		// Credentials aren't expired yet.
		return cfg.creds, nil
	}
	// Refresh credentials.
	newCreds, err := getCreds(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot refresh service account api token: %w", err)
	}
	cfg.creds = newCreds
	logger.Infof("yandexcloud_sd: successfully refreshed service account api token; expiration: %.3f seconds", time.Until(newCreds.Expiration).Seconds())
	return newCreds, nil
}

// getCreds get Yandex Cloud IAM token based on configuration
func getCreds(cfg *apiConfig) (*apiCredentials, error) {
	if cfg.yandexPassportOAuth == nil {
		return getInstanceCreds(cfg)
	}
	it, err := getIAMToken(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot get IAM token: %w", err)
	}
	return &apiCredentials{
		Token:      it.IAMToken,
		Expiration: it.ExpiresAt,
	}, nil
}

// getInstanceCreds gets Yandex Cloud IAM token using instance Service Account
//
// See https://cloud.yandex.com/en-ru/docs/compute/operations/vm-connect/auth-inside-vm
func getInstanceCreds(cfg *apiConfig) (*apiCredentials, error) {
	// Try obtaining GCE-like creds at first.
	// See https://yandex.cloud/en-ru/docs/compute/operations/vm-connect/auth-inside-vm#auth-inside-vm
	creds, err := getGCEInstanceCreds(cfg)
	if err == nil {
		return creds, nil
	}
	errGCE := err

	// Fall back to the disabled IMDSv1 - see https://yandex.cloud/en/docs/security/standard/authentication#aws-token
	//
	// TODO: remove this when it is completely removed from Yandex Cloud.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5513
	// and https://yandex.cloud/en/docs/security/standard/authentication#aws-token
	creds, err = getEC2IMDBSv1Creds(cfg)
	if err == nil {
		return creds, nil
	}

	// Return errGCE, since it is likely the IMDBSv1 is disabled.
	return nil, errGCE
}

// getGCEInstanceCreds gets Yandex Cloud IAM token using GCE API
//
// See https://yandex.cloud/en-ru/docs/compute/operations/vm-connect/auth-inside-vm#auth-inside-vm
func getGCEInstanceCreds(cfg *apiConfig) (*apiCredentials, error) {
	endpoint := "http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/token"
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		logger.Panicf("BUG: cannot create GCE token request for %s: %s", endpoint, err)
	}
	req.Header.Add("Metadata-Flavor", "Google")

	resp, err := cfg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain GCE token from %s: %w", endpoint, err)
	}
	data, err := readResponseBody(resp, endpoint)
	if err != nil {
		return nil, fmt.Errorf("cannot read GCE token from %s: %w", endpoint, err)
	}

	var ac gceAPICredentials
	if err := json.Unmarshal(data, &ac); err != nil {
		return nil, fmt.Errorf("cannot unmarshal GCE token from %s: %w; data=%s", endpoint, err, data)
	}
	if ac.TokenType != "Bearer" {
		return nil, fmt.Errorf("unsupported GCE token type received from %s: %q; supported: %q", endpoint, ac.TokenType, "Bearer")
	}

	expiration := time.Now().Add(time.Duration(ac.ExpiresIn) * time.Second)
	return &apiCredentials{
		Token:      ac.AccessToken,
		Expiration: expiration,
	}, nil
}

// See https://yandex.cloud/en-ru/docs/compute/operations/vm-connect/auth-inside-vm#auth-inside-vm
type gceAPICredentials struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// getEC2IMDBSv1Creds gets Yandex Cloud IAM token using Amazon EC2 IMDBSv1
func getEC2IMDBSv1Creds(cfg *apiConfig) (*apiCredentials, error) {
	endpoint := "http://169.254.169.254/latest/meta-data/iam/security-credentials/default"
	resp, err := cfg.client.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("cannot read Amazon EC2 IMDBSv1 token from %s: %w", endpoint, err)
	}
	data, err := readResponseBody(resp, endpoint)
	if err != nil {
		return nil, err
	}

	var ac apiCredentials
	if err := json.Unmarshal(data, &ac); err != nil {
		return nil, fmt.Errorf("cannot parse Amazon EC2 IMDBSv1 token from %s: %w; data=%s", endpoint, err, data)
	}
	return &ac, nil
}

// getIAMToken gets Yandex Cloud IAM token using OAuth
//
// See https://cloud.yandex.com/en-ru/docs/iam/operations/iam-token/create
func getIAMToken(cfg *apiConfig) (*iamToken, error) {
	iamURL := cfg.serviceEndpoints["iam"] + "/iam/v1/tokens"
	passport, err := json.Marshal(cfg.yandexPassportOAuth)
	if err != nil {
		logger.Panicf("BUG: cannot marshal yandex passport OAuth token: %s", err)
	}
	body := bytes.NewBuffer(passport)
	resp, err := cfg.client.Post(iamURL, "application/json", body)
	if err != nil {
		return nil, fmt.Errorf("cannot send request to yandex cloud iam api %q: %s", iamURL, err)
	}
	data, err := readResponseBody(resp, iamURL)
	if err != nil {
		return nil, err
	}
	var it iamToken
	if err := json.Unmarshal(data, &it); err != nil {
		return nil, fmt.Errorf("cannot parse iam token from %s: %w; data: %s", iamURL, err, data)
	}
	return &it, nil
}

// iamToken represents Yandex Cloud IAM token response
//
// See https://cloud.yandex.com/en-ru/docs/iam/operations/iam-token/create
type iamToken struct {
	IAMToken  string    `json:"iamToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// getServiceEndpoints returns services endpoints map
//
// See https://cloud.yandex.com/en-ru/docs/api-design-guide/concepts/endpoints
func (cfg *apiConfig) getServiceEndpoints(apiEndpoint string) (map[string]string, error) {
	apiEndpointURL, err := url.Parse(apiEndpoint)
	if err != nil {
		return nil, fmt.Errorf("cannot parse api_endpoint %q: %w", apiEndpoint, err)
	}
	scheme := apiEndpointURL.Scheme
	if scheme == "" {
		return nil, fmt.Errorf("missing scheme in api_endpoint %q", apiEndpoint)
	}
	if apiEndpointURL.Host == "" {
		return nil, fmt.Errorf("missing host in api_endpoint %q", apiEndpoint)
	}
	endpointsURL := apiEndpoint + "/endpoints"
	resp, err := cfg.client.Get(endpointsURL)
	if err != nil {
		return nil, fmt.Errorf("cannot query %q: %w", endpointsURL, err)
	}
	data, err := readResponseBody(resp, endpointsURL)
	if err != nil {
		return nil, err
	}
	var eps endpoints
	if err := json.Unmarshal(data, &eps); err != nil {
		return nil, fmt.Errorf("cannot parse API endpoints list: %w; data=%s", err, data)
	}
	m := make(map[string]string, len(eps.Endpoints))
	for _, endpoint := range eps.Endpoints {
		m[endpoint.ID] = scheme + "://" + endpoint.Address
	}
	return m, nil
}

type endpoints struct {
	Endpoints []endpoint `json:"endpoints"`
}

type endpoint struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

// getAPIResponse calls Yandex Cloud apiURL and returns response body.
func getAPIResponse(apiURL string, cfg *apiConfig) ([]byte, error) {
	creds, err := cfg.getFreshAPICredentials()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		logger.Panicf("BUG: cannot create new request for yandex cloud api url %s: %s", apiURL, err)
	}
	req.Header.Set("Authorization", "Bearer "+creds.Token)
	resp, err := cfg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot query yandex cloud api url %s: %w", apiURL, err)
	}
	return readResponseBody(resp, apiURL)
}

// readResponseBody reads body from http.Response.
func readResponseBody(resp *http.Response, apiURL string) ([]byte, error) {
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot read response from %q: %w", apiURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code for %q; got %d; want %d; response body: %s",
			apiURL, resp.StatusCode, http.StatusOK, data)
	}
	return data, nil
}
