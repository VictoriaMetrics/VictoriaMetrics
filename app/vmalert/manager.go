package main

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// manager controls group states
type manager struct {
	querierBuilder datasource.QuerierBuilder
	notifiers      []notifier.Notifier

	rw *remotewrite.Client
	// remote read builder.
	rr datasource.QuerierBuilder

	wg     sync.WaitGroup
	labels map[string]string

	groupsMu sync.RWMutex
	groups   map[uint64]*Group
}

// AlertAPI generates APIAlert object from alert by its ID(hash)
func (m *manager) AlertAPI(gID, aID uint64) (*APIAlert, error) {
	m.groupsMu.RLock()
	defer m.groupsMu.RUnlock()

	g, ok := m.groups[gID]
	if !ok {
		return nil, fmt.Errorf("can't find group with id %q", gID)
	}
	for _, rule := range g.Rules {
		ar, ok := rule.(*AlertingRule)
		if !ok {
			continue
		}
		if apiAlert := ar.AlertAPI(aID); apiAlert != nil {
			return apiAlert, nil
		}
	}
	return nil, fmt.Errorf("can't find alert with id %q in group %q", aID, g.Name)
}

func (m *manager) start(ctx context.Context, groupsCfg []config.Group) error {
	return m.update(ctx, groupsCfg, true)
}

func (m *manager) close() {
	if m.rw != nil {
		err := m.rw.Close()
		if err != nil {
			logger.Fatalf("cannot stop the remotewrite: %s", err)
		}
	}
	m.wg.Wait()
}

func (m *manager) startGroup(ctx context.Context, group *Group, restore bool) error {
	if restore && m.rr != nil {
		err := group.Restore(ctx, m.rr, *remoteReadLookBack, m.labels)
		if err != nil {
			if !*remoteReadIgnoreRestoreErrors {
				return fmt.Errorf("failed to restore state for group %q: %w", group.Name, err)
			}
			logger.Errorf("error while restoring state for group %q: %s", group.Name, err)
		}
	}

	m.wg.Add(1)
	id := group.ID()
	go func() {
		group.start(ctx, m.notifiers, m.rw)
		m.wg.Done()
	}()
	m.groups[id] = group
	return nil
}

func (m *manager) update(ctx context.Context, groupsCfg []config.Group, restore bool) error {
	var rrPresent, arPresent bool
	groupsRegistry := make(map[uint64]*Group)
	for _, cfg := range groupsCfg {
		for _, r := range cfg.Rules {
			if rrPresent && arPresent {
				continue
			}
			if r.Record != "" {
				rrPresent = true
			}
			if r.Alert != "" {
				arPresent = true
			}
		}
		ng := newGroup(cfg, m.querierBuilder, *evaluationInterval, m.labels)
		groupsRegistry[ng.ID()] = ng
	}

	if rrPresent && m.rw == nil {
		return fmt.Errorf("config contains recording rules but `-remoteWrite.url` isn't set")
	}
	if arPresent && m.notifiers == nil {
		return fmt.Errorf("config contains alerting rules but `-notifier.url` isn't set")
	}

	type updateItem struct {
		old *Group
		new *Group
	}
	var toUpdate []updateItem

	m.groupsMu.Lock()
	for _, og := range m.groups {
		ng, ok := groupsRegistry[og.ID()]
		if !ok {
			// old group is not present in new list,
			// so must be stopped and deleted
			og.close()
			delete(m.groups, og.ID())
			og = nil
			continue
		}
		delete(groupsRegistry, ng.ID())
		if og.Checksum != ng.Checksum {
			toUpdate = append(toUpdate, updateItem{old: og, new: ng})
		}
	}
	for _, ng := range groupsRegistry {
		if err := m.startGroup(ctx, ng, restore); err != nil {
			return err
		}
	}
	m.groupsMu.Unlock()

	if len(toUpdate) > 0 {
		var wg sync.WaitGroup
		for _, item := range toUpdate {
			wg.Add(1)
			go func(old *Group, new *Group) {
				old.updateCh <- new
				wg.Done()
			}(item.old, item.new)
		}
		wg.Wait()
	}
	return nil
}

func (g *Group) toAPI() APIGroup {
	g.mu.RLock()
	defer g.mu.RUnlock()

	ag := APIGroup{
		// encode as string to avoid rounding
		ID: fmt.Sprintf("%d", g.ID()),

		Name:        g.Name,
		Type:        g.Type.String(),
		File:        g.File,
		Interval:    g.Interval.String(),
		Concurrency: g.Concurrency,
		Params:      urlValuesToStrings(g.Params),
		Labels:      g.Labels,
	}
	for _, r := range g.Rules {
		switch v := r.(type) {
		case *AlertingRule:
			ag.AlertingRules = append(ag.AlertingRules, v.RuleAPI())
		case *RecordingRule:
			ag.RecordingRules = append(ag.RecordingRules, v.RuleAPI())
		}
	}
	return ag
}

func urlValuesToStrings(values url.Values) []string {
	if len(values) < 1 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var res []string
	for _, k := range keys {
		params := values[k]
		for _, v := range params {
			res = append(res, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return res
}
