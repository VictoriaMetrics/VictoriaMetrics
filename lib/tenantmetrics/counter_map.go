package tenantmetrics

import (
	"fmt"
	"sync"
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

	m sync.Map
	// mt holds value for multi-tenant metrics.
	mt atomic.Value
}

// NewCounterMap creates new CounterMap for the given metric.
func NewCounterMap(metric string) *CounterMap {
	return &CounterMap{
		metric: metric,
	}
}

// Get returns counter for the given at
func (cm *CounterMap) Get(at *auth.Token) *metrics.Counter {
	key := TenantID{
		AccountID: at.AccountID,
		ProjectID: at.ProjectID,
	}
	return cm.GetByTenant(&key)
}

// MultiAdd adds multiple values grouped by auth.Token
func (cm *CounterMap) MultiAdd(perTenantValues map[auth.Token]int) {
	for token, value := range perTenantValues {
		cm.Get(&token).Add(value)
	}
}

// GetByTenant returns counter for the given key.
func (cm *CounterMap) GetByTenant(key *TenantID) *metrics.Counter {
	if key == nil {
		mtm := cm.mt.Load()
		if mtm == nil {
			mtc := metrics.GetOrCreateCounter(createMetricNameMultitenant(cm.metric))
			cm.mt.Store(mtc)
			return mtc
		}
		return mtm.(*metrics.Counter)
	}

	if counter, ok := cm.m.Load(*key); ok {
		return counter.(*metrics.Counter)
	}

	// Slow path - create missing counter for k.
	metricName := createMetricName(cm.metric, *key)
	c := metrics.GetOrCreateCounter(metricName)
	cm.m.Store(*key, c)
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

func createMetricNameMultitenant(metric string) string {
	if len(metric) == 0 {
		logger.Panicf("BUG: metric cannot be empty")
	}
	if metric[len(metric)-1] != '}' {
		// Metric without labels.
		return fmt.Sprintf(`%s{accountID="multitenant",projectID="multitenant"}`, metric)
	}
	// Metric with labels.
	return fmt.Sprintf(`%s,accountID="multitenant",projectID="multitenant"}`, metric[:len(metric)-1])
}
