package yandexcloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

const (
	defaultInstanceCredsEndpoint = "http://169.254.169.254/latest/meta-data/iam/security-credentials/default"
	defaultAPIEndpoint           = "https://api.cloud.yandex.net"
	defaultAPIVersion            = "v1"
)

var configMap = discoveryutils.NewConfigMap()

type apiCredentials struct {
	Token      string    `json:"Token"`
	Expiration time.Time `json:"Expiration"`
}

// yandexPassportOAuth is a struct for Yandex Cloud IAM token request
// https://cloud.yandex.com/en-ru/docs/iam/operations/iam-token/create
type yandexPassportOAuth struct {
	YandexPassportOAuthToken string `json:"yandexPassportOauthToken"`
}

// iamToken Yandex Cloud IAM token response
// https://cloud.yandex.com/en-ru/docs/iam/operations/iam-token/create
type iamToken struct {
	IAMToken  string    `json:"iamToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type apiConfig struct {
	client              *http.Client
	tokenLock           sync.Mutex
	creds               *apiCredentials
	yandexPassportOAuth *yandexPassportOAuth
	serviceEndpoints    map[string]*url.URL
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	cfg := &apiConfig{
		client: &http.Client{
			Transport: &http.Transport{
				MaxIdleConnsPerHost: 100,
			},
		},
	}
	if sdc.TLSConfig != nil {
		opts := &promauth.Options{
			BaseDir:   baseDir,
			TLSConfig: sdc.TLSConfig,
		}
		ac, err := opts.NewConfig()
		if err != nil {
			return nil, err
		}
		cfg.client.Transport = &http.Transport{
			TLSClientConfig:     ac.NewTLSConfig(),
			MaxIdleConnsPerHost: 100,
		}
	}

	if err := cfg.getEndpoints(sdc.ApiEndpoint); err != nil {
		return nil, err
	}

	if sdc.YandexPassportOAuthToken != nil {
		logger.Infof("Using yandex passport OAuth token")

		cfg.yandexPassportOAuth = &yandexPassportOAuth{
			YandexPassportOAuthToken: sdc.YandexPassportOAuthToken.String(),
		}
	}

	return cfg, nil
}

// getFreshAPICredentials checks token lifetime and update if needed
func (cfg *apiConfig) getFreshAPICredentials() (*apiCredentials, error) {
	cfg.tokenLock.Lock()
	defer cfg.tokenLock.Unlock()

	if cfg.creds != nil && time.Until(cfg.creds.Expiration) > 10*time.Second {
		// Credentials aren't expired yet.
		return cfg.creds, nil
	}

	newCreds, err := getCreds(cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot refresh service account api token: %w", err)
	}
	cfg.creds = newCreds

	logger.Infof("successfully refreshed service account api token; expiration: %.3f seconds", time.Until(newCreds.Expiration).Seconds())

	return newCreds, nil
}

// getCreds get Yandex Cloud IAM token based on configuration
func getCreds(cfg *apiConfig) (*apiCredentials, error) {
	if cfg.yandexPassportOAuth == nil {
		return getInstanceCreds(cfg)
	}

	it, err := getIAMToken(cfg)
	if err != nil {
		return nil, err
	}

	return &apiCredentials{
		Token:      it.IAMToken,
		Expiration: it.ExpiresAt,
	}, nil
}

// getInstanceCreds gets Yandex Cloud IAM token using instance Service Account
// https://cloud.yandex.com/en-ru/docs/compute/operations/vm-connect/auth-inside-vm
func getInstanceCreds(cfg *apiConfig) (*apiCredentials, error) {
	resp, err := cfg.client.Get(defaultInstanceCredsEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed query security credentials api, url: %s, err: %w", defaultInstanceCredsEndpoint, err)
	}
	r, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot read response from %q: %w", defaultInstanceCredsEndpoint, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth failed, bad status code: %d, want: 200", resp.StatusCode)
	}

	var ac apiCredentials
	if err := json.Unmarshal(r, &ac); err != nil {
		return nil, fmt.Errorf("cannot parse auth credentials response: %w", err)
	}

	return &ac, nil
}

// getIAMToken gets Yandex Cloud IAM token using OAuth:
// https://cloud.yandex.com/en-ru/docs/iam/operations/iam-token/create
func getIAMToken(cfg *apiConfig) (*iamToken, error) {
	iamURL := *cfg.serviceEndpoints["iam"]
	iamURL.Path = path.Join(iamURL.Path, "iam", defaultAPIVersion, "tokens")

	passport, err := json.Marshal(cfg.yandexPassportOAuth)
	if err != nil {
		return nil, fmt.Errorf("failed marshall yandex passport OAuth token, err: %w", err)
	}

	resp, err := cfg.client.Post(iamURL.String(), "application/json", bytes.NewBuffer(passport))
	if err != nil {
		return nil, fmt.Errorf("failed query yandex cloud iam api, url: %s, err: %w", iamURL.String(), err)
	}

	r, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot read response from %q: %w", iamURL.String(), err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth failed, bad status code: %d, want: 200", resp.StatusCode)
	}

	it := iamToken{}
	if err := json.Unmarshal(r, &it); err != nil {
		return nil, fmt.Errorf("cannot parse auth credentials response: %w", err)
	}

	return &it, nil
}

// getEndpoints makes services endpoints map:
// https://cloud.yandex.com/en-ru/docs/api-design-guide/concepts/endpoints
func (cfg *apiConfig) getEndpoints(apiEndpoint string) error {
	if apiEndpoint == "" {
		apiEndpoint = defaultAPIEndpoint
	}

	apiEndpointURL, err := url.Parse(apiEndpoint)
	if err != nil {
		return fmt.Errorf("cannot parse api_endpoint: %s as url, err: %w", apiEndpoint, err)
	}

	apiEndpointURL.Path = path.Join(apiEndpointURL.Path, "endpoints")

	resp, err := cfg.client.Get(apiEndpointURL.String())
	if err != nil {
		return fmt.Errorf("failed query endpoints, url: %s, err: %w", apiEndpointURL.String(), err)
	}
	r, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return fmt.Errorf("cannot read response from %q: %w", apiEndpointURL.String(), err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth failed, bad status code: %d, want: 200", resp.StatusCode)
	}

	endpoints, err := parseEndpoints(r)
	if err != nil {
		return err
	}

	cfg.serviceEndpoints = make(map[string]*url.URL, len(endpoints.Endpoints))
	for _, endpoint := range endpoints.Endpoints {
		cfg.serviceEndpoints[endpoint.ID] = &url.URL{
			Scheme: apiEndpointURL.Scheme,
			Host:   endpoint.Address,
		}
	}

	return nil
}

// readResponseBody reads body from http.Response.
func readResponseBody(resp *http.Response, apiURL string) ([]byte, error) {
	data, err := ioutil.ReadAll(resp.Body)
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

// getAPIResponse calls Yandex Cloud apiURL and returns response body.
func getAPIResponse(apiURL string, cfg *apiConfig) ([]byte, error) {
	creds, err := cfg.getFreshAPICredentials()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create new request for openstack api url %s: %w", apiURL, err)
	}

	req.Header.Set("Authorization", "Bearer "+creds.Token)
	resp, err := cfg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot query yandex cloud api url %s: %w", apiURL, err)
	}

	return readResponseBody(resp, apiURL)
}
