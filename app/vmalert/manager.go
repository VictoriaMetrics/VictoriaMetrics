package main

import (
	"context"
	"fmt"
	"strconv"
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

	rw remotewrite.RWClient
	// remote read builder.
	rr datasource.QuerierBuilder

	wg     sync.WaitGroup
	labels map[string]string

	groupsMu sync.RWMutex
	groups   map[uint64]*rule.Group
}

// groupAPI generates apiGroup object from group by its ID(hash)
func (m *manager) groupAPI(gID uint64) (*rule.ApiGroup, error) {
	m.groupsMu.RLock()
	defer m.groupsMu.RUnlock()

	g, ok := m.groups[gID]
	if !ok {
		return nil, fmt.Errorf("can't find group with id %d", gID)
	}
	return g.ToAPI(), nil
}

// ruleAPI generates apiRule object from alert by its ID(hash)
func (m *manager) ruleAPI(gID, rID uint64) (rule.ApiRule, error) {
	m.groupsMu.RLock()
	defer m.groupsMu.RUnlock()

	group, ok := m.groups[gID]
	if !ok {
		return rule.ApiRule{}, fmt.Errorf("can't find group with id %d", gID)
	}
	g := group.ToAPI()
	ruleID := strconv.FormatUint(rID, 10)
	for _, r := range g.Rules {
		if r.ID == ruleID {
			return r, nil
		}
	}
	return rule.ApiRule{}, fmt.Errorf("can't find rule with id %d in group %q", rID, g.Name)
}

// alertAPI generates apiAlert object from alert by its ID(hash)
func (m *manager) alertAPI(gID, aID uint64) (*rule.ApiAlert, error) {
	m.groupsMu.RLock()
	defer m.groupsMu.RUnlock()

	group, ok := m.groups[gID]
	if !ok {
		return nil, fmt.Errorf("can't find group with id %d", gID)
	}
	g := group.ToAPI()
	for _, r := range g.Rules {
		if r.Type != rule.TypeAlerting {
			continue
		}
		alertID := strconv.FormatUint(aID, 10)
		for _, a := range r.Alerts {
			if a.ID == alertID {
				return a, nil
			}
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

func (m *manager) startGroup(ctx context.Context, g *rule.Group, restore bool) {
	id := g.GetID()
	g.Init()
	m.wg.Go(func() {
		if restore {
			g.Start(ctx, m.rw, m.rr)
		} else {
			g.Start(ctx, m.rw, nil)
		}
	})

	m.groups[id] = g
}

func (m *manager) update(ctx context.Context, groupsCfg []config.Group, restore bool) error {
	var rrPresent, arPresent bool
	groupsRegistry := make(map[uint64]*rule.Group)
	for _, cfg := range groupsCfg {
		for _, r := range cfg.Rules {
			if rrPresent && arPresent {
				break
			}
			if r.Record != "" {
				rrPresent = true
			}
			if r.Alert != "" {
				arPresent = true
			}
		}
		ng := rule.NewGroup(cfg, m.querierBuilder, *evaluationInterval, m.labels)
		groupsRegistry[ng.GetID()] = ng
	}

	if rrPresent && m.rw == nil {
		return fmt.Errorf("config contains recording rules but `-remoteWrite.url` isn't set")
	}
	if arPresent && notifier.GetTargets() == nil {
		return fmt.Errorf("config contains alerting rules but neither `-notifier.url` nor `-notifier.config` nor `-notifier.blackhole` aren't set")
	}

	type updateItem struct {
		old *rule.Group
		new *rule.Group
	}
	var toUpdate []updateItem

	m.groupsMu.Lock()
	for _, og := range m.groups {
		ng, ok := groupsRegistry[og.GetID()]
		if !ok {
			// old group is not present in new list,
			// so must be stopped and deleted
			og.Close()
			delete(m.groups, og.GetID())
			og = nil
			continue
		}
		delete(groupsRegistry, ng.GetID())
		if og.GetCheckSum() != ng.GetCheckSum() {
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
			oldG := item.old
			newG := item.new
			wg.Go(func() {
				// cancel evaluation so the Update will be applied as fast as possible.
				// it is important to call InterruptEval before the update, because cancel fn
				// can be re-assigned during the update.
				oldG.InterruptEval()
				oldG.UpdateWith(newG)
			})
		}
		wg.Wait()
	}
	return nil
}
