package tenantmetrics

import (
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

// TenantID defines metric tenant.
type TenantID struct {
	AccountID uint32
	ProjectID uint32
}

// CounterMap is a map of counters keyed by tenant.
type CounterMap struct {
	metric string

	// do not use atomic.Pointer, since the stored map there is already a pointer type.
	m atomic.Value
}

// NewCounterMap creates new CounterMap for the given metric.
func NewCounterMap(metric string) *CounterMap {
	cm := &CounterMap{
		metric: metric,
	}
	cm.m.Store(make(map[TenantID]*metrics.Counter))
	return cm
}

// Get returns counter for the given at
func (cm *CounterMap) Get(at *auth.Token) *metrics.Counter {
	key := TenantID{
		AccountID: at.AccountID,
		ProjectID: at.ProjectID,
	}
	return cm.GetByTenant(key)
}

// MultiAdd adds multiple values grouped by auth.Token
func (cm *CounterMap) MultiAdd(perTenantValues map[auth.Token]int) {
	for token, value := range perTenantValues {
		cm.Get(&token).Add(value)
	}
}

// GetByTenant returns counter for the given key.
func (cm *CounterMap) GetByTenant(key TenantID) *metrics.Counter {
	m := cm.m.Load().(map[TenantID]*metrics.Counter)
	if c := m[key]; c != nil {
		// Fast path - the counter for k already exists.
		return c
	}

	// Slow path - create missing counter for k and re-create m.
	newM := make(map[TenantID]*metrics.Counter, len(m)+1)
	for k, c := range m {
		newM[k] = c
	}
	metricName := createMetricName(cm.metric, key)
	c := metrics.GetOrCreateCounter(metricName)
	newM[key] = c
	cm.m.Store(newM)
	return c
}

func createMetricName(metric string, key TenantID) string {
	if len(metric) == 0 {
		logger.Panicf("BUG: metric cannot be empty")
	}
	if metric[len(metric)-1] != '}' {
		// Metric without labels.
		return fmt.Sprintf(`%s{accountID="%d",projectID="%d"}`, metric, key.AccountID, key.ProjectID)
	}
	// Metric with labels.
	return fmt.Sprintf(`%s,accountID="%d",projectID="%d"}`, metric[:len(metric)-1], key.AccountID, key.ProjectID)
}
