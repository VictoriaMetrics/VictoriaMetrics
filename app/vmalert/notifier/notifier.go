package notifier

import "context"

// Notifier is common interface for alert manager provider
type Notifier interface {
	Send(ctx context.Context, alerts []Alert) error
}
