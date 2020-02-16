package provider

import "github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"

// AlertManager represents integration provider with Prometheus alert manager
type AlertManager struct{}

// Send an alert or resolve message
func (a *AlertManager) Send(rule config.Alert) error {
	return nil
}
