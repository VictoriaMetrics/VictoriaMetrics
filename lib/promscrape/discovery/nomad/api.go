package nomad

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

var waitTime = flag.Duration("promscrape.nomad.waitTime", 0, "Wait time used by Nomad service discovery. Default value is used if not set")

// apiConfig contains config for API server.
type apiConfig struct {
	tagSeparator string
	nomadWatcher *nomadWatcher
}

func (ac *apiConfig) mustStop() {
	ac.nomadWatcher.mustStop()
}

var configMap = discoveryutil.NewConfigMap()

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	hcc := sdc.HTTPClientConfig
	token := os.Getenv("NOMAD_TOKEN")
	if token != "" {
		if hcc.BearerToken != nil {
			return nil, fmt.Errorf("cannot set both NOMAD_TOKEN and bearer_token")
		}
		hcc.BearerToken = promauth.NewSecret(token)
	}
	ac, err := hcc.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	apiServer := sdc.Server
	if apiServer == "" {
		apiServer = os.Getenv("NOMAD_ADDR")
		if apiServer == "" {
			apiServer = "localhost:4646"
		}
	}
	if !strings.Contains(apiServer, "://") {
		scheme := "http"
		if hcc.TLSConfig != nil {
			scheme = "https"
		}
		apiServer = scheme + "://" + apiServer
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}
	client, err := discoveryutil.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}
	tagSeparator := ","
	if sdc.TagSeparator != nil {
		tagSeparator = *sdc.TagSeparator
	}

	namespace := sdc.Namespace
	if namespace == "" {
		namespace = os.Getenv("NOMAD_NAMESPACE")
	}

	region := sdc.Region
	if region == "" {
		region = os.Getenv("NOMAD_REGION")
		if region == "" {
			region = "global"
		}
	}

	nw := newNomadWatcher(client, sdc, namespace, region)
	cfg := &apiConfig{
		tagSeparator: tagSeparator,
		nomadWatcher: nw,
	}
	return cfg, nil
}

// maxWaitTime is duration for Nomad blocking request.
func maxWaitTime() time.Duration {
	d := discoveryutil.BlockingClientReadTimeout
	// Nomad adds random delay up to wait/16, so reduce the timeout in order to keep it below BlockingClientReadTimeout.
	// See https://developer.hashicorp.com/nomad/api-docs#blocking-queries
	d -= d / 16
	// The timeout cannot exceed 10 minutes. See https://developer.hashicorp.com/nomad/api-docs#blocking-queries

	if d > 10*time.Minute {
		d = 10 * time.Minute
	}
	if *waitTime > time.Second && *waitTime < d {
		d = *waitTime
	}
	return d
}

// getBlockingAPIResponse performs blocking request to Nomad via client and returns response.
// See https://developer.hashicorp.com/nomad/api-docs#blocking-queries .
func getBlockingAPIResponse(ctx context.Context, client *discoveryutil.Client, path string, index int64) ([]byte, int64, error) {
	path += "&index=" + strconv.FormatInt(index, 10)
	path += "&wait=" + fmt.Sprintf("%ds", int(maxWaitTime().Seconds()))
	getMeta := func(resp *http.Response) {
		if resp.StatusCode != http.StatusOK {
			return
		}
		ind := resp.Header.Get("X-Nomad-Index")
		if len(ind) == 0 {
			logger.Errorf("cannot find X-Nomad-Index header in response from %q", path)
			return
		}
		newIndex, err := strconv.ParseInt(ind, 10, 64)
		if err != nil {
			logger.Errorf("cannot parse X-Nomad-Index header value in response from %q: %s", path, err)
			return
		}
		// Properly handle the returned newIndex according to https://developer.hashicorp.com/nomad/api-docs#blocking-queries.
		// Index implementation details are the same for Consul and Nomad: https://developer.hashicorp.com/consul/api-docs/features/blocking#implementation-details
		if newIndex < 1 {
			index = 1
			return
		}
		if index > newIndex {
			index = 0
			return
		}
		index = newIndex
	}
	data, err := client.GetBlockingAPIResponseCtx(ctx, path, getMeta)
	if err != nil {
		return nil, index, fmt.Errorf("cannot perform blocking Nomad API request at %q: %w", path, err)
	}
	return data, index, nil
}
