package provider

import "github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"

// AlertProvider is common interface for alert manager provider
type AlertProvider interface {
	Send(rule config.Alert) error
}
