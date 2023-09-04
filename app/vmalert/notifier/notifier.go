package notifier

import "context"

// Notifier is a common interface for alert manager provider
type Notifier interface {
	// Send sends the given list of alerts.
	// Returns an error if fails to send the alerts.
	// Must unblock if the given ctx is cancelled.
	Send(ctx context.Context, alerts []Alert, notifierHeaders map[string]string) error
	// Addr returns address where alerts are sent.
	Addr() string
	// Close is a destructor for the Notifier
	Close()
}
