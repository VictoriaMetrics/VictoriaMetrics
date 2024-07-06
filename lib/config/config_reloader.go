package config

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fscore"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	configPath          = flag.String("runtime.config", "", "path to config")
	configCheckInterval = flag.Duration("runtime.configCheckInterval", 10*time.Second, "interval for config file re-read. "+
		"Zero value disables config re-reading. By default, refreshing is disabled, send SIGHUP for config refresh.")
)

var (
	configLoader func(data []byte) error
	configData   atomic.Pointer[[]byte]
	stopCh       chan struct{}
	configWg     sync.WaitGroup

	configReloads      = metrics.NewCounter(`vm_runtime_config_last_reload_total`)
	configReloadErrors = metrics.NewCounter(`vm_runtime_config_last_reload_errors_total`)
	configSuccess      = metrics.NewGauge(`vm_runtime_config_last_reload_successful`, nil)
	configTimestamp    = metrics.NewCounter(`vm_runtime_config_last_reload_success_timestamp_seconds`)
)

func LoadConfig(runtimeConfigLoader func(data []byte) error) (context.CancelFunc, error) {
	if len(*configPath) == 0 {
		logger.Fatalf("missing required `-config` command-line flag")
	}

	configLoader = runtimeConfigLoader
	sighupCh := procutil.NewSighupChan()
	_, err := loadConfig()
	if err != nil {
		logger.Fatalf("cannot load auth config: %s", err)
	}

	configSuccess.Set(1)
	configTimestamp.Set(fasttime.UnixTimestamp())

	stopCh = make(chan struct{})
	configWg.Add(1)
	go func() {
		defer configWg.Done()
		configReloader(sighupCh)
	}()
	return stopReloadConfig, nil
}

func loadConfig() (bool, error) {
	data, err := fscore.ReadFileOrHTTP(*configPath)
	if err != nil {
		return false, fmt.Errorf("failed to read -auth.config=%q: %w", *configPath, err)
	}

	oldData := configData.Load()
	if oldData != nil && bytes.Equal(data, *oldData) {
		// there are no updates in the config - skip reloading.
		return false, nil
	}

	err = configLoader(data)
	if err != nil {
		return false, fmt.Errorf("failed to parse -auth.config=%q: %w", *configPath, err)
	}

	configData.Store(&data)
	return true, nil
}

func stopReloadConfig() {
	close(stopCh)
	configWg.Wait()
	logger.Infof("stopped config reloader")
}

func configReloader(sighupCh <-chan os.Signal) {
	var refreshCh <-chan time.Time
	// initialize auth refresh interval
	if *configCheckInterval > 0 {
		ticker := time.NewTicker(*configCheckInterval)
		defer ticker.Stop()
		refreshCh = ticker.C
	}

	updateFn := func() {
		configReloads.Inc()
		updated, err := loadConfig()
		if err != nil {
			logger.Errorf("failed to load auth config; using the last successfully loaded config; error: %s", err)
			configSuccess.Set(0)
			configReloadErrors.Inc()
			return
		}
		configSuccess.Set(1)
		if updated {
			configTimestamp.Set(fasttime.UnixTimestamp())
			logger.Infof("successfully reloaded auth config")
		}
	}

	for {
		select {
		case <-stopCh:
			return
		case <-refreshCh:
			updateFn()
		case <-sighupCh:
			logger.Infof("SIGHUP received; loading -runtime.config=%q", *configPath)
			updateFn()
		}
	}
}
