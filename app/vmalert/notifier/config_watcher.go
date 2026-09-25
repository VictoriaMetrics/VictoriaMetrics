package notifier

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/consul"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discovery/dns"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// configWatcher supports dynamic reload of Notifier objects
// from static configuration and service discovery.
// Use newWatcher to create a new object.
type configWatcher struct {
	genFn AlertURLGenerator

	reloadCh chan struct{}

	// tw holds notifiers for the last successfully applied config
	tw atomic.Pointer[targetsWatcher]
}

// targetsWatcher discovers and holds Notifier objects for a single config.
// Use startTargetsWatcher to create a new object.
type targetsWatcher struct {
	cfg   *Config
	genFn AlertURLGenerator
	wg    sync.WaitGroup

	syncCh chan struct{}

	targetsMu sync.RWMutex
	targets   map[TargetType][]Target
}

func newWatcher(cfg *Config, gen AlertURLGenerator) (*configWatcher, error) {
	tw, err := startTargetsWatcher(cfg, gen)
	if err != nil {
		return nil, err
	}
	cw := &configWatcher{
		genFn:    gen,
		reloadCh: make(chan struct{}, 1),
	}
	cw.tw.Store(tw)
	return cw, nil
}

// startTargetsWatcher starts discovery of notifiers for the given cfg.
//
// If it fails, all the resources allocated for cfg are released.
func startTargetsWatcher(cfg *Config, gen AlertURLGenerator) (*targetsWatcher, error) {
	tw := &targetsWatcher{
		cfg:     cfg,
		genFn:   gen,
		syncCh:  make(chan struct{}),
		targets: make(map[TargetType][]Target),
	}
	if err := tw.start(); err != nil {
		tw.mustStop()
		return nil, err
	}
	return tw, nil
}

func (cw *configWatcher) notifiers() []Notifier {
	tw := cw.tw.Load()
	tw.targetsMu.RLock()
	defer tw.targetsMu.RUnlock()

	var notifiers []Notifier
	for _, ns := range tw.targets {
		for _, n := range ns {
			notifiers = append(notifiers, n.Notifier)
		}
	}
	// deterministically sort the output
	sort.Slice(notifiers, func(i, j int) bool {
		return notifiers[i].Addr() < notifiers[j].Addr()
	})
	return notifiers
}

// getTargets returns a copy of targets for the currently applied config.
func (cw *configWatcher) getTargets() map[TargetType][]Target {
	tw := cw.tw.Load()
	tw.targetsMu.RLock()
	defer tw.targetsMu.RUnlock()

	targets := make(map[TargetType][]Target, len(tw.targets))
	for key, ns := range tw.targets {
		targets[key] = append(targets[key], ns...)
	}
	return targets
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
	twOld := cw.tw.Load()
	if cfg.Checksum == twOld.cfg.Checksum {
		return nil
	}

	// start notifiers for the new config before stopping the existing ones,
	// so notifiers of the previous config keep working if the new config fails to start.
	// The failed config is retried on the next reload, since its checksum differs from the applied one.
	tw, err := startTargetsWatcher(cfg, cw.genFn)
	if err != nil {
		return err
	}
	cw.tw.Store(tw)
	twOld.mustStop()
	return nil
}

func (cw *configWatcher) mustStop() {
	cw.tw.Load().mustStop()
}

func (tw *targetsWatcher) add(typeK TargetType, interval time.Duration, targetsFn getTargets) error {
	targetMetadata, errors := getTargetMetadata(targetsFn, tw.cfg)
	for _, err := range errors {
		return fmt.Errorf("failed to init notifier for %q: %w", typeK, err)
	}

	tw.updateTargets(typeK, targetMetadata, tw.cfg, tw.genFn)

	tw.wg.Go(func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-tw.syncCh:
				return
			case <-ticker.C:
			}
			targetMetadata, errors := getTargetMetadata(targetsFn, tw.cfg)
			for _, err := range errors {
				logger.Errorf("failed to init notifier for %q: %s", typeK, err)
			}
			tw.updateTargets(typeK, targetMetadata, tw.cfg, tw.genFn)
		}
	})
	return nil
}

type targetMetadata struct {
	*promutil.Labels
	alertRelabelConfigs *promrelabel.ParsedConfigs
}

func getTargetMetadata(targetsFn getTargets, cfg *Config) (map[string]targetMetadata, []error) {
	metaLabelsList, alertRelabelCfgs, err := targetsFn()
	if err != nil {
		return nil, []error{fmt.Errorf("failed to get labels: %w", err)}
	}
	targetMts := make(map[string]targetMetadata, len(metaLabelsList))
	var errors []error
	duplicates := make(map[string]struct{})
	for i := range metaLabelsList {
		metaLabels := metaLabelsList[i]
		alertRelabelCfg := alertRelabelCfgs[i]
		for _, labels := range metaLabels {
			target := labels.Get("__address__")
			u, processedLabels, err := parseLabels(target, labels, cfg)
			if err != nil {
				errors = append(errors, err)
				continue
			}
			if len(u) == 0 {
				continue
			}
			// check for duplicated targets
			// targets with same address but different alert_relabel_configs are still considered duplicates since it's mostly due to misconfiguration and could cause duplicated notifications.
			if _, ok := duplicates[u]; ok {
				if !*suppressDuplicateTargetErrors {
					logger.Errorf("skipping duplicate target with identical address %q; "+
						"make sure service discovery and relabeling is set up properly; "+
						"original labels: %s; resulting labels: %s",
						u, labels, processedLabels)
				}
				continue
			}
			duplicates[u] = struct{}{}
			targetMts[u] = targetMetadata{
				Labels:              processedLabels,
				alertRelabelConfigs: alertRelabelCfg,
			}
		}
	}
	return targetMts, errors
}

type getTargets func() ([][]*promutil.Labels, []*promrelabel.ParsedConfigs, error)

func (tw *targetsWatcher) start() error {
	if len(tw.cfg.StaticConfigs) > 0 {
		var targets []Target
		// closeTargets releases resources of already created targets,
		// since they aren't registered in tw.targets and cannot be closed by mustStop.
		closeTargets := func() {
			for _, t := range targets {
				t.Close()
			}
		}
		for i, cfg := range tw.cfg.StaticConfigs {
			alertRelabelConfig, _ := promrelabel.ParseRelabelConfigs(tw.cfg.StaticConfigs[i].AlertRelabelConfigs)
			httpCfg := mergeHTTPClientConfigs(tw.cfg.HTTPClientConfig, cfg.HTTPClientConfig)
			for _, target := range cfg.Targets {
				address, labels, err := parseLabels(target, nil, tw.cfg)
				if err != nil {
					closeTargets()
					return fmt.Errorf("failed to parse labels for target %q: %w", target, err)
				}
				notifier, err := NewAlertManager(address, tw.genFn, httpCfg, alertRelabelConfig, tw.cfg.Timeout.Duration())
				if err != nil {
					closeTargets()
					return fmt.Errorf("failed to init alertmanager for addr %q: %w", address, err)
				}
				targets = append(targets, Target{
					Notifier: notifier,
					Labels:   labels,
				})
			}
		}
		tw.setTargets(TargetStatic, targets)
	}

	if len(tw.cfg.ConsulSDConfigs) > 0 {
		err := tw.add(TargetConsul, *consul.SDCheckInterval, func() ([][]*promutil.Labels, []*promrelabel.ParsedConfigs, error) {
			var labels [][]*promutil.Labels
			var alertRelabelConfigs []*promrelabel.ParsedConfigs
			for i := range tw.cfg.ConsulSDConfigs {
				alertRelabelConfig, _ := promrelabel.ParseRelabelConfigs(tw.cfg.ConsulSDConfigs[i].AlertRelabelConfigs)
				sdc := &tw.cfg.ConsulSDConfigs[i]
				targetLabels, err := sdc.GetLabels(tw.cfg.baseDir)
				if err != nil {
					return nil, nil, fmt.Errorf("got labels err: %w", err)
				}
				labels = append(labels, targetLabels)
				alertRelabelConfigs = append(alertRelabelConfigs, alertRelabelConfig)
			}
			return labels, alertRelabelConfigs, nil
		})
		if err != nil {
			return fmt.Errorf("failed to start consulSD discovery: %w", err)
		}
	}

	if len(tw.cfg.DNSSDConfigs) > 0 {
		err := tw.add(TargetDNS, *dns.SDCheckInterval, func() ([][]*promutil.Labels, []*promrelabel.ParsedConfigs, error) {
			var labels [][]*promutil.Labels
			var alertRelabelConfigs []*promrelabel.ParsedConfigs
			for i := range tw.cfg.DNSSDConfigs {
				alertRelabelConfig, _ := promrelabel.ParseRelabelConfigs(tw.cfg.DNSSDConfigs[i].AlertRelabelConfigs)
				sdc := &tw.cfg.DNSSDConfigs[i]
				targetLabels, err := sdc.GetLabels(tw.cfg.baseDir)
				if err != nil {
					return nil, nil, fmt.Errorf("got labels err: %w", err)
				}
				labels = append(labels, targetLabels)
				alertRelabelConfigs = append(alertRelabelConfigs, alertRelabelConfig)

			}
			return labels, alertRelabelConfigs, nil
		})
		if err != nil {
			return fmt.Errorf("failed to start DNSSD discovery: %w", err)
		}
	}
	return nil
}

func (tw *targetsWatcher) mustStop() {
	close(tw.syncCh)
	tw.wg.Wait()

	tw.targetsMu.Lock()
	for _, targets := range tw.targets {
		for _, t := range targets {
			t.Close()
		}
	}
	tw.targets = make(map[TargetType][]Target)
	tw.targetsMu.Unlock()

	for i := range tw.cfg.ConsulSDConfigs {
		tw.cfg.ConsulSDConfigs[i].MustStop()
	}
	tw.cfg = nil
}

func (tw *targetsWatcher) setTargets(key TargetType, targets []Target) {
	tw.targetsMu.Lock()
	tw.targets[key] = targets
	tw.targetsMu.Unlock()
}

func (tw *targetsWatcher) updateTargets(key TargetType, targetMts map[string]targetMetadata, cfg *Config, genFn AlertURLGenerator) {
	tw.targetsMu.Lock()
	defer tw.targetsMu.Unlock()
	oldTargets := tw.targets[key]
	var updatedTargets []Target
	for _, ot := range oldTargets {
		if _, ok := targetMts[ot.Addr()]; !ok {
			// if target not exists in currentTargets, close it
			ot.Close()
		} else {
			updatedTargets = append(updatedTargets, ot)
			delete(targetMts, ot.Addr())
		}
	}
	// create new resources for the new targets
	for addr, metadata := range targetMts {
		am, err := NewAlertManager(addr, genFn, cfg.HTTPClientConfig, metadata.alertRelabelConfigs, cfg.Timeout.Duration())
		if err != nil {
			logger.Errorf("failed to init %s notifier with addr %q: %s", key, addr, err)
			continue
		}
		updatedTargets = append(updatedTargets, Target{
			Notifier: am,
			Labels:   metadata.Labels,
		})
	}

	tw.targets[key] = updatedTargets
}

// mergeHTTPClientConfigs merges fields between child and parent params
// by populating child from parent params if they're missing.
func mergeHTTPClientConfigs(parent, child promauth.HTTPClientConfig) promauth.HTTPClientConfig {
	if child.Authorization == nil {
		child.Authorization = parent.Authorization
	}
	if child.BasicAuth == nil {
		child.BasicAuth = parent.BasicAuth
	}
	if child.BearerToken == nil {
		child.BearerToken = parent.BearerToken
	}
	if child.BearerTokenFile == "" {
		child.BearerTokenFile = parent.BearerTokenFile
	}
	if child.OAuth2 == nil {
		child.OAuth2 = parent.OAuth2
	}
	if child.TLSConfig == nil {
		child.TLSConfig = parent.TLSConfig
	}
	if child.Headers == nil {
		child.Headers = parent.Headers
	}
	return child
}
