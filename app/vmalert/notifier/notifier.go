package notifier

// Notifier is common interface for alert manager provider
type Notifier interface {
	Send(alerts []Alert) error
}
