package nacos

import (
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

var configMap = discoveryutil.NewConfigMap()

func (ac *apiConfig) mustStop() {
	ac.nacosWatcher.mustStop()
}

func (nw *nacosWatcher) mustStop() {
	nw.client.Stop()
	<-nw.stoppedCh
}

// apiConfig contains config for API server.
type apiConfig struct {
	nacosWatcher *nacosWatcher
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	hcc := sdc.HTTPClientConfig
	ac, err := hcc.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	apiServer := sdc.Server
	if apiServer == "" {
		apiServer = "localhost:8848"
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

	if sdc.Namespace == "" {
		sdc.Namespace = "public"
	}

	if sdc.Group == "" {
		sdc.Group = "DEFAULT_GROUP"
	}

	if sdc.Cluster == "" {
		sdc.Cluster = "DEFAULT"
	}

	nw := newNacosWatcher(client, sdc)
	cfg := &apiConfig{
		nacosWatcher: nw,
	}
	return cfg, nil
}
