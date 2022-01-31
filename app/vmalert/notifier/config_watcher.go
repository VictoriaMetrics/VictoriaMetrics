package notifier

import (
	"fmt"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
)

// configWatcher supports dynamic reload of Notifier objects
// from static configuration and service discovery.
// Use newWatcher to create a new object.
type configWatcher struct {
	cfg   *Config
	genFn AlertURLGenerator
	wg    sync.WaitGroup

	reloadCh chan struct{}
	syncCh   chan struct{}

	targetsMu sync.RWMutex
	targets   map[TargetType][]Target
}

func newWatcher(path string, gen AlertURLGenerator) (*configWatcher, error) {
	cfg, err := parseConfig(path)
	if err != nil {
		return nil, err
	}
	cw := &configWatcher{
		cfg:       cfg,
		wg:        sync.WaitGroup{},
		reloadCh:  make(chan struct{}, 1),
		syncCh:    make(chan struct{}),
		genFn:     gen,
		targetsMu: sync.RWMutex{},
		targets:   make(map[TargetType][]Target),
	}
	return cw, cw.start()
}

func (cw *configWatcher) notifiers() []Notifier {
	cw.targetsMu.RLock()
	defer cw.targetsMu.RUnlock()

	var notifiers []Notifier
	for _, ns := range cw.targets {
		for _, n := range ns {
			notifiers = append(notifiers, n.Notifier)
		}

	}
	return notifiers
}

func (cw *configWatcher) reload(path string) error {
	select {
	case cw.reloadCh <- struct{}{}:
	default:
		return nil
	}

	defer func() { <-cw.reloadCh }()

	cfg, err := parseConfig(path)
	if err != nil {
		return err
	}
	if cfg.Checksum == cw.cfg.Checksum {
		return nil
	}

	// stop existing discovery
	close(cw.syncCh)
	cw.wg.Wait()

	// re-start cw with new config
	cw.syncCh = make(chan struct{})
	cw.cfg = cfg

	cw.targetsMu.Lock()
	cw.targets = make(map[TargetType][]Target)
	cw.targetsMu.Unlock()

	return cw.start()
}

const (
	addRetryBackoff = time.Millisecond * 100
	addRetryCount   = 2
)

func (cw *configWatcher) add(typeK TargetType, interval time.Duration, labelsFn getLabels) error {
	var targets []Target
	var errors []error
	var count int
	for { // retry addRetryCount times if first discovery attempts gave no results
		targets, errors = targetsFromLabels(labelsFn, cw.cfg, cw.genFn)
		for _, err := range errors {
			return fmt.Errorf("failed to init notifier for %q: %s", typeK, err)
		}
		if len(targets) > 0 || count >= addRetryCount {
			break
		}
		time.Sleep(addRetryBackoff)
	}

	cw.targetsMu.Lock()
	cw.targets[typeK] = targets
	cw.targetsMu.Unlock()

	cw.wg.Add(1)
	go func() {
		defer cw.wg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-cw.syncCh:
				return
			case <-ticker.C:
			}
			updateTargets, errors := targetsFromLabels(labelsFn, cw.cfg, cw.genFn)
			for _, err := range errors {
				logger.Errorf("failed to init notifier for %q: %s", typeK, err)
			}

			cw.targetsMu.Lock()
			cw.targets[typeK] = updateTargets
			cw.targetsMu.Unlock()
		}
	}()
	return nil
}

func targetsFromLabels(labelsFn getLabels, cfg *Config, genFn AlertURLGenerator) ([]Target, []error) {
	metaLabels, err := labelsFn()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to get labels: %s", err)}
	}

	var targets []Target
	var errors []error
	duplicates := make(map[string]struct{})
	for _, labels := range metaLabels {
		target := labels["__address__"]
		u, processedLabels, err := parseLabels(target, labels, cfg)
		if err != nil {
			errors = append(errors, err)
			continue
		}

		// check for duplicates
		if _, ok := duplicates[u]; ok {
			errors = append(errors, fmt.Errorf("duplicated url %q detected", u))
			continue
		}
		duplicates[u] = struct{}{}

		am, err := NewAlertManager(u, genFn, cfg.HTTPClientConfig, cfg.Timeout.Duration())
		if err != nil {
			errors = append(errors, err)
			continue
		}
		targets = append(targets, Target{
			Notifier: am,
			Labels:   processedLabels,
		})
	}
	return targets, errors
}

type getLabels func() ([]map[string]string, error)

func (cw *configWatcher) start() error {
	if len(cw.cfg.StaticConfigs) > 0 {
		cw.targetsMu.Lock()
		for _, cfg := range cw.cfg.StaticConfigs {
			for _, target := range cfg.Targets {
				address, labels, err := parseLabels(target, nil, cw.cfg)
				if err != nil {
					return fmt.Errorf("failed to parse labels for target %q: %s", target, err)
				}
				notifier, err := NewAlertManager(address, cw.genFn, cw.cfg.HTTPClientConfig, cw.cfg.Timeout.Duration())
				if err != nil {
					return fmt.Errorf("failed to init alertmanager for addr %q: %s", address, err)
				}
				cw.targets[TargetStatic] = append(cw.targets[TargetStatic], Target{
					Notifier: notifier,
					Labels:   labels,
				})
			}
		}
		cw.targetsMu.Unlock()
	}

	if len(cw.cfg.ConsulSDConfigs) > 0 {
		err := cw.add(TargetConsul, *consul.SDCheckInterval, func() ([]map[string]string, error) {
			var labels []map[string]string
			for i := range cw.cfg.ConsulSDConfigs {
				sdc := &cw.cfg.ConsulSDConfigs[i]
				targetLabels, err := sdc.GetLabels(cw.cfg.baseDir)
				if err != nil {
					return nil, fmt.Errorf("got labels err: %s", err)
				}
				labels = append(labels, targetLabels...)
			}
			return labels, nil
		})
		if err != nil {
			return fmt.Errorf("failed to start consulSD discovery: %s", err)
		}
	}
	return nil
}
