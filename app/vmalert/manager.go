package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type manager struct {
	storage  datasource.Querier
	notifier notifier.Notifier

	rw *remotewrite.Client
	rr datasource.Querier

	wg sync.WaitGroup

	groupsMu sync.RWMutex
	groups   map[uint64]*Group
}

// AlertAPI generates APIAlert object from alert by its id(hash)
func (m *manager) AlertAPI(gID, aID uint64) (*APIAlert, error) {
	m.groupsMu.RLock()
	defer m.groupsMu.RUnlock()

	g, ok := m.groups[gID]
	if !ok {
		return nil, fmt.Errorf("can't find group with id %q", gID)
	}
	for _, rule := range g.Rules {
		if apiAlert := rule.AlertAPI(aID); apiAlert != nil {
			return apiAlert, nil
		}
	}
	return nil, fmt.Errorf("can't func alert with id %q in group %q", aID, g.Name)
}

func (m *manager) start(ctx context.Context, path []string, validate bool) error {
	return m.update(ctx, path, validate, true)
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

func (m *manager) startGroup(ctx context.Context, group Group, restore bool) {
	if restore && m.rr != nil {
		err := group.Restore(ctx, m.rr, *remoteReadLookBack)
		if err != nil {
			logger.Errorf("error while restoring state for group %q: %s", group.Name, err)
		}
	}


	m.wg.Add(1)
	id := group.ID()
	go func() {
		group.start(ctx, group.Interval, m.storage, m.notifier, m.rw)
		m.wg.Done()
	}()
	m.groups[id] = &group
}

func (m *manager) update(ctx context.Context, path []string, validate, restore bool) error {
	logger.Infof("reading alert rules configuration file from %q", strings.Join(path, ";"))
	newGroups, err := Parse(path, validate)
	if err != nil {
		return fmt.Errorf("cannot parse configuration file: %s", err)
	}

	groupsRegistry := make(map[uint64]Group)
	for _, ng := range newGroups {
		groupsRegistry[ng.ID()] = ng
	}

	m.groupsMu.Lock()
	for _, og := range m.groups {
		id := og.ID()
		ng, ok := groupsRegistry[id]
		if !ok {
			// old group is not present in new list
			// and must be stopped and deleted
			og.close()
			delete(m.groups, og.ID())
			og = nil
			continue
		}
		og.updateWith(ng)
		delete(groupsRegistry, ng.ID())
	}

	for _, ng := range groupsRegistry {
		m.startGroup(ctx, ng, restore)
	}
	m.groupsMu.Unlock()
	return nil
}
