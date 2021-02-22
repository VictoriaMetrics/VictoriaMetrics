package kubernetes

import (
	"context"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// listMetadata kubernetes list metadata
// https://v1-17.docs.kubernetes.io/docs/reference/generated/kubernetes-api/v1.17/#listmeta-v1-meta
type listMetadata struct {
	ResourceVersion string `json:"resourceVersion"`
}

// apiConfig contains config for API server
type apiConfig struct {
	setName    string
	namespaces []string
	selectors  []Selector
	wc         *watchClient
	targetChan chan K8sSyncEvent
	watchOnce  sync.Once
}

var configMap = discoveryutils.NewConfigMap()

func getAPIConfig(ctx context.Context, wg *sync.WaitGroup, workChan chan K8sSyncEvent, setName string, sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(ctx, wg, workChan, setName, sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(ctx context.Context, wg *sync.WaitGroup, workChan chan K8sSyncEvent, setName string, sdc *SDConfig, baseDir string) (*apiConfig, error) {
	wc, err := newWatchClient(wg, sdc, baseDir)
	if err != nil {
		return nil, err
	}
	cfg := &apiConfig{
		setName:    setName,
		targetChan: workChan,
		wc:         wc,
		namespaces: sdc.Namespaces.Names,
		selectors:  sdc.Selectors,
	}
	return cfg, nil
}
