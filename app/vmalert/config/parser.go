package config

// AlertRule basic alert entity rule
type AlertRule struct{}

// Alerts grouping array of alert
type Alerts struct{}

// Parse parses config from given file
func Parse(filepath string) Alerts {
	return Alerts{}
}
