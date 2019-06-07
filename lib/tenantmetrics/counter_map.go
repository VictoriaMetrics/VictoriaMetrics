package tenantmetrics

import (
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

type apKey struct {
	accountID uint32
	projectID uint32
}

// CounterMap is a map of counters keyed by tenant.
type CounterMap struct {
	metric string
	m      atomic.Value
}

// NewCounterMap creates new CounterMap for the given metricTemplate.
//
func NewCounterMap(metric string) *CounterMap {
	cm := &CounterMap{
		metric: metric,
	}
	cm.m.Store(make(map[apKey]*metrics.Counter))
	return cm
}

// Get returns counter for the given at.
//
// It always returns non-nil counter.
func (cm *CounterMap) Get(at *auth.Token) *metrics.Counter {
	key := apKey{
		accountID: at.AccountID,
		projectID: at.ProjectID,
	}
	m := cm.m.Load().(map[apKey]*metrics.Counter)
	if c := m[key]; c != nil {
		// Fast path - the counter for k already exists.
		return c
	}

	// Slow path - create missing counter for k and re-create m.
	newM := make(map[apKey]*metrics.Counter, len(m)+1)
	for k, c := range m {
		newM[k] = c
	}
	metricName := createMetricName(cm.metric, at)
	c := metrics.GetOrCreateCounter(metricName)
	newM[key] = c
	cm.m.Store(newM)
	return c
}

func createMetricName(metric string, at *auth.Token) string {
	if len(metric) == 0 {
		logger.Panicf("BUG: metric cannot be empty")
	}
	if metric[len(metric)-1] != '}' {
		// Metric without labels.
		return fmt.Sprintf(`%s{accountID="%d",projectID="%d"}`, metric, at.AccountID, at.ProjectID)
	}
	// Metric with labels.
	return fmt.Sprintf(`%s,accountID="%d",projectID="%d"}`, metric[:len(metric)-1], at.AccountID, at.ProjectID)
}
