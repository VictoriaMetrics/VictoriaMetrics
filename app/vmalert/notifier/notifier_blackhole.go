package notifier

import "context"

// BlackHoleNotifier is used when no notifications needs to be sent
type BlackHoleNotifier struct {
	addr    string
	metrics *metrics
}

// Send will not send any notifications. Only increase the alerts sent number.
func (bh *BlackHoleNotifier) Send(_ context.Context, alerts []Alert, _ map[string]string) error { //nolint:revive
	bh.metrics.alertsSent.Add(len(alerts))
	return nil
}

// Addr of black hole notifier
func (bh BlackHoleNotifier) Addr() string {
	return bh.addr
}

// Close unregister the metrics
func (bh *BlackHoleNotifier) Close() {
	bh.metrics.alertsSent.Unregister()
	bh.metrics.alertsSendErrors.Unregister()
}

// NewBlackHoleNotifier Create a new BlackHoleNotifier
func NewBlackHoleNotifier() *BlackHoleNotifier {
	address := "blackhole"
	return &BlackHoleNotifier{
		addr:    address,
		metrics: newMetrics(address),
	}
}
