package notifier

import "context"

// blackHoleNotifier is a Notifier stub, used when no notifications need
// to be sent.
type blackHoleNotifier struct {
	addr    string
	metrics *metrics
}

// Send will send no notifications, but increase the metric.
func (bh *blackHoleNotifier) Send(_ context.Context, alerts []Alert, _ map[string]string) error { //nolint:revive
	bh.metrics.alertsSent.Add(len(alerts))
	return nil
}

// Addr of black hole notifier
func (bh blackHoleNotifier) Addr() string {
	return bh.addr
}

// Close unregister the metrics
func (bh *blackHoleNotifier) Close() {
	bh.metrics.alertsSent.Unregister()
	bh.metrics.alertsSendErrors.Unregister()
}

// newBlackHoleNotifier creates a new blackHoleNotifier
func newBlackHoleNotifier() *blackHoleNotifier {
	address := "blackhole"
	return &blackHoleNotifier{
		addr:    address,
		metrics: newMetrics(address),
	}
}
