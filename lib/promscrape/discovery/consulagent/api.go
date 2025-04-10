package consulagent

import (
	"fmt"
	"os"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

// apiConfig contains config for API server.
type apiConfig struct {
	tagSeparator  string
	consulWatcher *consulAgentWatcher
	agent         *consul.Agent
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
	token, err := consul.GetToken(sdc.Token)
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
	agent, err := consul.GetAgentInfo(client)
	if err != nil {
		client.Stop()
		return nil, fmt.Errorf("cannot obtain consul datacenter: %w", err)
	}
	dc := sdc.Datacenter
	if dc == "" {
		dc = agent.Config.Datacenter
	}

	namespace := sdc.Namespace
	// default namespace can be detected from env var.
	if namespace == "" {
		namespace = os.Getenv("CONSUL_NAMESPACE")
	}

	cw := newConsulAgentWatcher(client, sdc, dc, namespace)
	cfg := &apiConfig{
		tagSeparator:  tagSeparator,
		consulWatcher: cw,
		agent:         agent,
	}
	return cfg, nil
}
