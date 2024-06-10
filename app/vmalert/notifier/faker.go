package notifier

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FakeNotifier is a mock notifier
type FakeNotifier struct {
	sync.Mutex
	alerts []Alert
	// records number of received alerts in total
	counter int
}

// Close does nothing
func (*FakeNotifier) Close() {}

// Addr returns ""
func (*FakeNotifier) Addr() string { return "" }

// Send sets alerts and increases counter
func (fn *FakeNotifier) Send(_ context.Context, alerts []Alert, _ map[string]string) error {
	fn.Lock()
	defer fn.Unlock()
	fn.counter += len(alerts)
	fn.alerts = alerts
	return nil
}

// GetCounter returns received alerts count
func (fn *FakeNotifier) GetCounter() int {
	fn.Lock()
	defer fn.Unlock()
	return fn.counter
}

// GetAlerts returns stored alerts
func (fn *FakeNotifier) GetAlerts() []Alert {
	fn.Lock()
	defer fn.Unlock()
	return fn.alerts
}

// FaultyNotifier is a mock notifier that Send() will return failed response
type FaultyNotifier struct {
	FakeNotifier
}

// Send returns failed response
func (fn *FaultyNotifier) Send(ctx context.Context, _ []Alert, _ map[string]string) error {
	d, ok := ctx.Deadline()
	if ok {
		time.Sleep(time.Until(d))
	}
	return fmt.Errorf("send failed")
}
