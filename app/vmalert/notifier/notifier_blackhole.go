package notifier

import (
	"context"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
)

type blackHoleMetrics struct {
	alertsSent *utils.Counter
}

func newBlackHoleMetrics(addr string) *blackHoleMetrics {
	return &blackHoleMetrics{
		alertsSent: utils.GetOrCreateCounter(fmt.Sprintf("vmalert_alerts_sent_total{addr=%q}", addr)),
	}
}

// BlackHoleNotifier No op notifier which can be used when no notifications needs to be sent out
type BlackHoleNotifier struct {
	addr    string
	metrics *blackHoleMetrics
}

// Send will not send any notifications. Just update the number of alert it needs to sent.
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
}

// NewBlackHoleNotifier Create a new BlackHoleNotifier
func NewBlackHoleNotifier() *BlackHoleNotifier {
	address := "blackhole"
	return &BlackHoleNotifier{
		addr:    address,
		metrics: newBlackHoleMetrics(address),
	}
}
