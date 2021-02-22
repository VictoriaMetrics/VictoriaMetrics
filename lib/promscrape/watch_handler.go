package promscrape

import (
	"context"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/kubernetes"
)

type kubernetesWatchHandler struct {
	ctx       context.Context
	cancel    context.CancelFunc
	startOnce sync.Once
	watchCfg  *kubernetes.WatchConfig
	// guards cache and set
	mu             sync.Mutex
	lastAccessTime time.Time
	swCache        map[string][]*ScrapeWork
	sdcSet         map[string]*scrapeWorkConfig
}

func newKubernetesWatchHandler() *kubernetesWatchHandler {
	ctx, cancel := context.WithCancel(context.Background())
	kwh := &kubernetesWatchHandler{
		ctx:      ctx,
		cancel:   cancel,
		swCache:  map[string][]*ScrapeWork{},
		sdcSet:   map[string]*scrapeWorkConfig{},
		watchCfg: kubernetes.NewWatchConfig(ctx),
	}
	go kwh.waitForStop()
	return kwh
}

func (ksc *kubernetesWatchHandler) waitForStop() {
	t := time.NewTicker(time.Second * 5)
	for range t.C {
		ksc.mu.Lock()
		lastTime := time.Since(ksc.lastAccessTime)
		ksc.mu.Unlock()
		if lastTime > *kubernetesSDCheckInterval*30 {
			t1 := time.Now()
			ksc.cancel()
			ksc.watchCfg.WG.Wait()
			close(ksc.watchCfg.WatchChan)
			logger.Infof("stopped kubernetes api watcher handler, after: %.3f seconds", time.Since(t1).Seconds())
			ksc.watchCfg.SC = nil
			t.Stop()
			return
		}
	}
}

func processKubernetesSyncEvents(cfg *Config) {
	for {
		select {
		case <-cfg.kwh.ctx.Done():
			return
		case se, ok := <-cfg.kwh.watchCfg.WatchChan:
			if !ok {
				return
			}
			if se.Labels == nil {
				cfg.kwh.mu.Lock()
				delete(cfg.kwh.swCache, se.Key)
				cfg.kwh.mu.Unlock()
				continue
			}
			cfg.kwh.mu.Lock()
			swc, ok := cfg.kwh.sdcSet[se.ConfigSectionSet]
			cfg.kwh.mu.Unlock()
			if !ok {
				logger.Fatalf("bug config section not found: %v", se.ConfigSectionSet)
			}
			ms := appendScrapeWorkForTargetLabels(nil, swc, se.Labels, "kubernetes_sd_config")
			cfg.kwh.mu.Lock()
			cfg.kwh.swCache[se.Key] = ms
			cfg.kwh.mu.Unlock()
		}
	}
}
