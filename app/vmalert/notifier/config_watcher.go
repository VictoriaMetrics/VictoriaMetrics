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
	cw.mustStop()

	// re-start cw with new config
	cw.syncCh = make(chan struct{})
	cw.cfg = cfg
	return cw.start()
}

func (cw *configWatcher) add(typeK TargetType, interval time.Duration, labelsFn getLabels) error {
	targets, errors := targetsFromLabels(labelsFn, cw.cfg, cw.genFn)
	for _, err := range errors {
		return fmt.Errorf("failed to init notifier for %q: %s", typeK, err)
	}

	cw.setTargets(typeK, targets)

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
			cw.setTargets(typeK, updateTargets)
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
		if len(u) == 0 {
			continue
		}
		if _, ok := duplicates[u]; ok { // check for duplicates
			if !*suppressDuplicateTargetErrors {
				logger.Errorf("skipping duplicate target with identical address %q; "+
					"make sure service discovery and relabeling is set up properly; "+
					"original labels: %s; resulting labels: %s",
					u, labels, processedLabels)
			}
			continue
		}
		duplicates[u] = struct{}{}

		am, err := NewAlertManager(u, genFn, cfg.HTTPClientConfig, cfg.parsedAlertRelabelConfigs, cfg.Timeout.Duration())
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
		var targets []Target
		for _, cfg := range cw.cfg.StaticConfigs {
			for _, target := range cfg.Targets {
				address, labels, err := parseLabels(target, nil, cw.cfg)
				if err != nil {
					return fmt.Errorf("failed to parse labels for target %q: %s", target, err)
				}
				notifier, err := NewAlertManager(address, cw.genFn, cw.cfg.HTTPClientConfig, cw.cfg.parsedRelabelConfigs, cw.cfg.Timeout.Duration())
				if err != nil {
					return fmt.Errorf("failed to init alertmanager for addr %q: %s", address, err)
				}
				targets = append(targets, Target{
					Notifier: notifier,
					Labels:   labels,
				})
			}
		}
		cw.setTargets(TargetStatic, targets)
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

func (cw *configWatcher) mustStop() {
	close(cw.syncCh)
	cw.wg.Wait()

	cw.targetsMu.Lock()
	for _, targets := range cw.targets {
		for _, t := range targets {
			t.Close()
		}
	}
	cw.targets = make(map[TargetType][]Target)
	cw.targetsMu.Unlock()

	for i := range cw.cfg.ConsulSDConfigs {
		cw.cfg.ConsulSDConfigs[i].MustStop()
	}
	cw.cfg = nil
}

func (cw *configWatcher) setTargets(key TargetType, targets []Target) {
	cw.targetsMu.Lock()
	newT := make(map[string]Target)
	for _, t := range targets {
		newT[t.Addr()] = t
	}
	oldT := cw.targets[key]

	for _, ot := range oldT {
		if _, ok := newT[ot.Addr()]; !ok {
			ot.Notifier.Close()
		}
	}
	cw.targets[key] = targets
	cw.targetsMu.Unlock()
}
