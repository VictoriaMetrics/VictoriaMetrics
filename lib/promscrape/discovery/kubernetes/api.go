package kubernetes

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// apiConfig contains config for API server
type apiConfig struct {
	setName    string
	namespaces []string
	selectors  []Selector
	wc         *watchClient
	targetChan chan SyncEvent
	watchOnce  sync.Once
}

var configMap = discoveryutils.NewConfigMap()

func getAPIConfig(watchCfg *WatchConfig, setName string, sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(watchCfg, setName, sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(watchCfg *WatchConfig, setName string, sdc *SDConfig, baseDir string) (*apiConfig, error) {
	wc, err := newWatchClient(watchCfg.WG, sdc, baseDir)
	if err != nil {
		return nil, err
	}
	cfg := &apiConfig{
		setName:    setName,
		targetChan: watchCfg.WatchChan,
		wc:         wc,
		namespaces: sdc.Namespaces.Names,
		selectors:  sdc.Selectors,
	}
	return cfg, nil
}
