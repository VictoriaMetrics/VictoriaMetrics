package rule

import (
	"context"
	"flag"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

func TestExecutorRetriesFailedNotifications(t *testing.T) {
	oldResendDelay := *resendDelay
	*resendDelay = time.Hour
	t.Cleanup(func() { *resendDelay = oldResendDelay })
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()
	initRetryNotifierForTest(t, srv.URL)
	fq := &datasource.FakeQuerier{}
	fq.Add(metricWithValueAndLabels(t, 1, "job", "bar"))
	ar := newTestAlertingRule("test", 0)
	ar.q = fq
	e := &executor{}
	ts := time.Now()
	if err := e.exec(context.Background(), ar, ts, time.Hour, 0, nil); err == nil {
		t.Fatal("expected notifier failure")
	}
	for _, a := range ar.alerts {
		if !a.LastSent.IsZero() {
			t.Fatal("failed delivery updated LastSent")
		}
	}
	if err := e.exec(context.Background(), ar, ts, time.Hour, 0, nil); err != nil {
		t.Fatal(err)
	}
	for _, a := range ar.alerts {
		if a.LastSent.IsZero() {
			t.Fatal("successful delivery did not update LastSent")
		}
	}
	if attempts.Load() != 2 {
		t.Fatalf("got %d delivery attempts; want 2", attempts.Load())
	}
	if err := e.exec(context.Background(), ar, ts, time.Hour, 0, nil); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 2 {
		t.Fatal("successful delivery was retried before resend delay")
	}
}

func TestExecutorRetriesCanceledNotification(t *testing.T) {
	// Use a real HTTP notifier so cancellation is returned by its client.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }))
	defer srv.Close()
	initRetryNotifierForTest(t, srv.URL)
	fq := &datasource.FakeQuerier{}
	fq.Add(metricWithValueAndLabels(t, 1, "job", "bar"))
	ar := newTestAlertingRule("test", 0)
	ar.q = fq
	e := &executor{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := e.exec(ctx, ar, time.Now(), time.Hour, 0, nil); err == nil {
		t.Fatal("expected canceled delivery error")
	}
	for _, a := range ar.alerts {
		if !a.LastSent.IsZero() {
			t.Fatal("canceled delivery updated LastSent")
		}
	}
	if err := e.exec(context.Background(), ar, time.Now(), time.Hour, 0, nil); err != nil {
		t.Fatal(err)
	}
	for _, a := range ar.alerts {
		if a.LastSent.IsZero() {
			t.Fatal("canceled notification was not retried")
		}
	}
}

func initRetryNotifierForTest(t *testing.T, addr string) {
	t.Helper()
	_, restoreNotifiers := notifier.InitFakeNotifier()
	t.Cleanup(restoreNotifiers)
	urls := flag.Lookup("notifier.url").Value.(*flagutil.ArrayString)
	oldURLs := *urls
	oldGenerator := notifier.AlertURLGeneratorFn
	t.Cleanup(func() { *urls = oldURLs; notifier.AlertURLGeneratorFn = oldGenerator })
	*urls = flagutil.ArrayString{addr}
	notifier.AlertURLGeneratorFn = func(notifier.Alert) string { return "" }
	if err := notifier.Init(nil, ""); err != nil {
		t.Fatal(err)
	}
}
