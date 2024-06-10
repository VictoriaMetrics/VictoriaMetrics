package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// manager controls group states
type manager struct {
	querierBuilder datasource.QuerierBuilder
	notifiers      func() []notifier.Notifier

	rw remotewrite.RWClient
	// remote read builder.
	rr datasource.QuerierBuilder

	wg     sync.WaitGroup
	labels map[string]string

	groupsMu sync.RWMutex
	groups   map[uint64]*rule.Group
}

// ruleAPI generates apiRule object from alert by its ID(hash)
func (m *manager) ruleAPI(gID, rID uint64) (apiRule, error) {
	m.groupsMu.RLock()
	defer m.groupsMu.RUnlock()

	g, ok := m.groups[gID]
	if !ok {
		return apiRule{}, fmt.Errorf("can't find group with id %d", gID)
	}
	for _, rule := range g.Rules {
		if rule.ID() == rID {
			return ruleToAPI(rule), nil
		}
	}
	return apiRule{}, fmt.Errorf("can't find rule with id %d in group %q", rID, g.Name)
}

// alertAPI generates apiAlert object from alert by its ID(hash)
func (m *manager) alertAPI(gID, aID uint64) (*apiAlert, error) {
	m.groupsMu.RLock()
	defer m.groupsMu.RUnlock()

	g, ok := m.groups[gID]
	if !ok {
		return nil, fmt.Errorf("can't find group with id %d", gID)
	}
	for _, r := range g.Rules {
		ar, ok := r.(*rule.AlertingRule)
		if !ok {
			continue
		}
		if apiAlert := alertToAPI(ar, aID); apiAlert != nil {
			return apiAlert, nil
		}
	}
	return nil, fmt.Errorf("can't find alert with id %d in group %q", aID, g.Name)
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

func (m *manager) startGroup(ctx context.Context, g *rule.Group, restore bool) error {
	m.wg.Add(1)
	id := g.ID()
	go func() {
		defer m.wg.Done()
		if restore {
			g.Start(ctx, m.notifiers, m.rw, m.rr)
		} else {
			g.Start(ctx, m.notifiers, m.rw, nil)
		}
	}()
	m.groups[id] = g
	return nil
}

func (m *manager) update(ctx context.Context, groupsCfg []config.Group, restore bool) error {
	var rrPresent, arPresent bool
	groupsRegistry := make(map[uint64]*rule.Group)
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
		ng := rule.NewGroup(cfg, m.querierBuilder, *evaluationInterval, m.labels)
		groupsRegistry[ng.ID()] = ng
	}

	if rrPresent && m.rw == nil {
		return fmt.Errorf("config contains recording rules but `-remoteWrite.url` isn't set")
	}
	if arPresent && m.notifiers == nil {
		return fmt.Errorf("config contains alerting rules but neither `-notifier.url` nor `-notifier.config` nor `-notifier.blackhole` aren't set")
	}

	type updateItem struct {
		old *rule.Group
		new *rule.Group
	}
	var toUpdate []updateItem

	m.groupsMu.Lock()
	for _, og := range m.groups {
		ng, ok := groupsRegistry[og.ID()]
		if !ok {
			// old group is not present in new list,
			// so must be stopped and deleted
			og.Close()
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
			m.groupsMu.Unlock()
			return err
		}
	}
	m.groupsMu.Unlock()

	if len(toUpdate) > 0 {
		var wg sync.WaitGroup
		for _, item := range toUpdate {
			wg.Add(1)
			// cancel evaluation so the Update will be applied as fast as possible.
			// it is important to call InterruptEval before the update, because cancel fn
			// can be re-assigned during the update.
			item.old.InterruptEval()
			go func(old *rule.Group, new *rule.Group) {
				old.UpdateWith(new)
				wg.Done()
			}(item.old, item.new)
		}
		wg.Wait()
	}
	return nil
}
