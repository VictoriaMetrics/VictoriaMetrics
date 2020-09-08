package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// manager controls group states
type manager struct {
	querier   datasource.Querier
	notifiers []notifier.Notifier

	rw *remotewrite.Client
	rr datasource.Querier

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

func (m *manager) start(ctx context.Context, path []string, validateTpl, validateExpr bool) error {
	return m.update(ctx, path, validateTpl, validateExpr, true)
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

func (m *manager) startGroup(ctx context.Context, group *Group, restore bool) {
	if restore && m.rr != nil {
		err := group.Restore(ctx, m.rr, *remoteReadLookBack, m.labels)
		if err != nil {
			logger.Errorf("error while restoring state for group %q: %s", group.Name, err)
		}
	}

	m.wg.Add(1)
	id := group.ID()
	go func() {
		group.start(ctx, m.querier, m.notifiers, m.rw)
		m.wg.Done()
	}()
	m.groups[id] = group
}

func (m *manager) update(ctx context.Context, path []string, validateTpl, validateExpr, restore bool) error {
	logger.Infof("reading rules configuration file from %q", strings.Join(path, ";"))
	groupsCfg, err := config.Parse(path, validateTpl, validateExpr)
	if err != nil {
		return fmt.Errorf("cannot parse configuration file: %w", err)
	}

	groupsRegistry := make(map[uint64]*Group)
	for _, cfg := range groupsCfg {
		ng := newGroup(cfg, *evaluationInterval, m.labels)
		groupsRegistry[ng.ID()] = ng
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
		m.startGroup(ctx, ng, restore)
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
		ID:          fmt.Sprintf("%d", g.ID()),
		Name:        g.Name,
		File:        g.File,
		Interval:    g.Interval.String(),
		Concurrency: g.Concurrency,
	}
	ag.AuthToken = nil
	if g.at != nil {
		ag.AuthToken = &APIAuthToken{
			AccountID: g.at.AccountID,
			ProjectID: g.at.ProjectID,
		}
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
