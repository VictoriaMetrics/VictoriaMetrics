package provider

import "github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"

// AlertManager represents integration provider with Prometheus alert manager
type AlertManager struct{}

// Fire fires an alert
func (a *AlertManager) Fire(rule config.AlertRule) error {
	return nil
}
