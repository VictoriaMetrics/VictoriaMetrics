package storage

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
)

type key struct {
	name  string
	group string
}

type MemoryStorage struct {
	m    sync.RWMutex
	data map[key]*config.Rule
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		data: make(map[key]*config.Rule),
	}
}

func (m *MemoryStorage) Save(a *config.Rule) {
	m.m.Lock()
	m.data[key{
		name:  a.Name,
		group: a.Group,
	}] = a
	m.m.Unlock()
}

func (m *MemoryStorage) Get(group, name string) *config.Rule {
	m.m.RLock()
	defer m.m.RUnlock()
	return m.data[key{
		name:  name,
		group: group,
	}]
}

func (m *MemoryStorage) Delete(group, name string) {
	m.m.Lock()
	delete(m.data, key{
		name:  group,
		group: name,
	})
	m.m.Unlock()
}
