package puppetdb

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

var configMap = discoveryutil.NewConfigMap()

type apiConfig struct {
	client *discoveryutil.Client

	query             string
	includeParameters bool
	port              int
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	// the following param checks align with Prometheus
	if sdc.URL == "" {
		return nil, errors.New("URL is missing")
	}
	parsedURL, err := url.Parse(sdc.URL)
	if err != nil {
		return nil, fmt.Errorf("parse URL %s error: %v", sdc.URL, err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("URL %s scheme must be 'http' or 'https'", sdc.URL)
	}
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("host is missing in URL %s", sdc.URL)
	}
	if sdc.Query == "" {
		return nil, errors.New("query missing")
	}

	port := sdc.Port
	if port == 0 {
		port = 80
	}

	// other general checks
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}

	client, err := discoveryutil.NewClient(parsedURL.String(), ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", sdc.URL, err)
	}

	return &apiConfig{
		client: client,

		query:             sdc.Query,
		includeParameters: sdc.IncludeParameters,
		port:              port,
	}, nil
}
