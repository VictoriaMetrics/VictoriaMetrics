package discoveryutil

import (
	"sync"

	"github.com/VictoriaMetrics/metrics"
)

// ConfigMap is a map for storing discovery api configs.
//
// It automatically removes old configs which weren't accessed recently.
type ConfigMap struct {
	mu sync.Mutex
	m  map[any]any

	entriesCount *metrics.Counter
}

// NewConfigMap creates ConfigMap
func NewConfigMap() *ConfigMap {
	return &ConfigMap{
		m:            make(map[any]any),
		entriesCount: metrics.GetOrCreateCounter(`vm_promscrape_discoveryutil_configmap_entries_count`),
	}
}

// Get returns config for the given key.
//
// Key must be a pointer.
//
// It creates new config map with newConfig() call if cm doesn't contain config under the given key.
func (cm *ConfigMap) Get(key any, newConfig func() (any, error)) (any, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cfg := cm.m[key]
	if cfg != nil {
		return cfg, nil
	}
	cfg, err := newConfig()
	if err != nil {
		return nil, err
	}
	cm.m[key] = cfg
	cm.entriesCount.Inc()
	return cfg, nil
}

// Delete deletes config for the given key from cm and returns it.
func (cm *ConfigMap) Delete(key any) any {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cfg := cm.m[key]
	if cfg == nil {
		// The cfg can be missing if it wasn't accessed yet.
		return nil
	}
	cm.entriesCount.Dec()
	delete(cm.m, key)
	return cfg
}
