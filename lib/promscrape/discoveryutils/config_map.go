package discoveryutils

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// ConfigMap is a map for storing discovery api configs.
//
// It automatically removes old configs which weren't accessed recently.
type ConfigMap struct {
	mu         sync.Mutex
	m          map[interface{}]*configMapEntry
	hasCleaner bool
}

// NewConfigMap creates ConfigMap
func NewConfigMap() *ConfigMap {
	return &ConfigMap{
		m: make(map[interface{}]*configMapEntry),
	}
}

// Get returns config for the given key.
//
// Key must be a pointer.
//
// It creates new config map with newConfig() call if cm doesn't contain config under the given key.
func (cm *ConfigMap) Get(key interface{}, newConfig func() (interface{}, error)) (interface{}, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.hasCleaner {
		cm.hasCleaner = true
		go cm.cleaner()
	}

	e := cm.m[key]
	if e != nil {
		e.lastAccessTime = fasttime.UnixTimestamp()
		return e.cfg, nil
	}
	cfg, err := newConfig()
	if err != nil {
		return nil, err
	}
	cm.m[key] = &configMapEntry{
		cfg:            cfg,
		lastAccessTime: fasttime.UnixTimestamp(),
	}
	return cfg, nil
}

func (cm *ConfigMap) cleaner() {
	tc := time.NewTicker(15 * time.Minute)
	for range tc.C {
		currentTime := fasttime.UnixTimestamp()
		cm.mu.Lock()
		for k, e := range cm.m {
			if currentTime-e.lastAccessTime > 10*60 {
				delete(cm.m, k)
			}
		}
		cm.mu.Unlock()
	}
}

type configMapEntry struct {
	cfg            interface{}
	lastAccessTime uint64
}
