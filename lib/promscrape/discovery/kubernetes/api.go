package kubernetes

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// apiConfig contains config for API server
type apiConfig struct {
	client     *discoveryutils.Client
	namespaces []string
	selectors  []Selector
	wc         *watchClient
	// holds main watched objects.
	watchCache             sync.Map
	mu                     sync.Mutex
	servicesLastAccessTime time.Time
}

var configMap = discoveryutils.NewConfigMap()

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	wc, err := newWatchClient(sdc, "")
	if err != nil {
		return nil, err
	}
	cfg := &apiConfig{
		wc:                     wc,
		namespaces:             sdc.Namespaces.Names,
		selectors:              sdc.Selectors,
		servicesLastAccessTime: time.Now(),
	}
	if err := startWatcherByRole(sdc.Role, cfg, sdc); err != nil {
		return nil, err
	}
	go stopWatcher(cfg)
	return cfg, nil
}

func getAPIResponse(cfg *apiConfig, role, path string) ([]byte, error) {
	query := joinSelectors(role, cfg.namespaces, cfg.selectors)
	if len(query) > 0 {
		path += "?" + query
	}
	return cfg.client.GetAPIResponse(path)
}
