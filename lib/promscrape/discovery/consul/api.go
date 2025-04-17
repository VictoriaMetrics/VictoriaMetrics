package consul

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

var waitTime = flag.Duration("promscrape.consul.waitTime", 0, "Wait time used by Consul service discovery. Default value is used if not set")

// apiConfig contains config for API server.
type apiConfig struct {
	tagSeparator  string
	consulWatcher *consulWatcher
}

func (ac *apiConfig) mustStop() {
	ac.consulWatcher.mustStop()
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
	token, err := GetToken(sdc.Token)
	if err != nil {
		return nil, err
	}
	if token != "" {
		if hcc.BearerToken != nil {
			return nil, fmt.Errorf("cannot set both token and bearer_token configs")
		}
		hcc.BearerToken = promauth.NewSecret(token)
	}
	if len(sdc.Username) > 0 {
		if hcc.BasicAuth != nil {
			return nil, fmt.Errorf("cannot set both username and basic_auth configs")
		}
		hcc.BasicAuth = &promauth.BasicAuthConfig{
			Username: sdc.Username,
			Password: sdc.Password,
		}
	}
	ac, err := hcc.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	apiServer := sdc.Server
	if apiServer == "" {
		apiServer = "localhost:8500"
	}
	if !strings.Contains(apiServer, "://") {
		scheme := sdc.Scheme
		if scheme == "" {
			scheme = "http"
			if hcc.TLSConfig != nil {
				scheme = "https"
			}
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
	dc, err := getDatacenter(client, sdc.Datacenter)
	if err != nil {
		client.Stop()
		return nil, fmt.Errorf("cannot obtain consul datacenter: %w", err)
	}

	namespace := sdc.Namespace
	// default namespace can be detected from env var.
	if namespace == "" {
		namespace = os.Getenv("CONSUL_NAMESPACE")
	}

	cw := newConsulWatcher(client, sdc, dc, namespace)
	cfg := &apiConfig{
		tagSeparator:  tagSeparator,
		consulWatcher: cw,
	}
	return cfg, nil
}

// GetToken returns Consul token.
func GetToken(token *promauth.Secret) (string, error) {
	if token != nil {
		return token.String(), nil
	}
	if tokenFile := os.Getenv("CONSUL_HTTP_TOKEN_FILE"); tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			return "", fmt.Errorf("cannot read consul token file %q; probably, `token` arg is missing in `consul_sd_config`? error: %w", tokenFile, err)
		}
		return string(data), nil
	}
	t := os.Getenv("CONSUL_HTTP_TOKEN")
	// Allow empty token - it should work if authorization is disabled in Consul
	return t, nil
}

// GetAgentInfo returns information about current consul agent.
func GetAgentInfo(client *discoveryutil.Client) (*Agent, error) {
	// See https://www.consul.io/api/agent.html#read-configuration
	data, err := client.GetAPIResponse("/v1/agent/self")
	if err != nil {
		return nil, fmt.Errorf("cannot query consul agent info: %w", err)
	}
	a, err := ParseAgent(data)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func getDatacenter(client *discoveryutil.Client, dc string) (string, error) {
	if dc != "" {
		return dc, nil
	}
	agent, err := GetAgentInfo(client)
	if err != nil {
		return "", err
	}
	return agent.Config.Datacenter, nil
}

// maxWaitTime is duration for consul blocking request.
func maxWaitTime() time.Duration {
	d := discoveryutil.BlockingClientReadTimeout
	// Consul adds random delay up to wait/16, so reduce the timeout in order to keep it below BlockingClientReadTimeout.
	// See https://www.consul.io/api-docs/features/blocking
	d -= d / 8
	// The timeout cannot exceed 10 minutes. See https://www.consul.io/api-docs/features/blocking
	if d > 10*time.Minute {
		d = 10 * time.Minute
	}
	if *waitTime > time.Second && *waitTime < d {
		d = *waitTime
	}
	return d
}

// getBlockingAPIResponse performs blocking request to Consul via client and returns response.
//
// See https://www.consul.io/api-docs/features/blocking .
func getBlockingAPIResponse(ctx context.Context, client *discoveryutil.Client, path string, index int64) ([]byte, int64, error) {
	path += "&index=" + strconv.FormatInt(index, 10)
	path += "&wait=" + fmt.Sprintf("%ds", int(maxWaitTime().Seconds()))
	getMeta := func(resp *http.Response) {
		if resp.StatusCode != http.StatusOK {
			return
		}
		ind := resp.Header.Get("X-Consul-Index")
		if len(ind) == 0 {
			logger.Errorf("cannot find X-Consul-Index header in response from %q", path)
			return
		}
		newIndex, err := strconv.ParseInt(ind, 10, 64)
		if err != nil {
			logger.Errorf("cannot parse X-Consul-Index header value in response from %q: %s", path, err)
			return
		}
		// Properly handle the returned newIndex according to https://www.consul.io/api-docs/features/blocking#implementation-details
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
		return nil, index, fmt.Errorf("cannot perform blocking Consul API request at %q: %w", path, err)
	}
	return data, index, nil
}
