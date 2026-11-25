package rule

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

func TestLimitResolveSeries(t *testing.T) {
	for _, absent := range []bool{false, true} {
		t.Run(map[bool]string{false: "present", true: "absent"}[absent], func(t *testing.T) {
			ar, fq, id := newLimitResolveRule(t)
			if absent {
				fq.Reset()
				fq.Add(metricWithValueAndLabels(t, 1, "job", "baz"), metricWithValueAndLabels(t, 1, "job", "qux"))
			}
			ts := time.Now()
			tss, err := ar.exec(context.Background(), ts, 1, nil)
			if !errors.Is(err, errExecLimitExceeded) {
				t.Fatalf("expected limit error; got %v", err)
			}
			if len(tss) != 2 {
				t.Fatalf("got %d series; want exactly two stale resolve series", len(tss))
			}
			seen := make(map[string]bool)
			for _, series := range tss {
				var name, job string
				for _, label := range series.Labels {
					if label.Name == "__name__" {
						name = label.Value
					}
					if label.Name == "job" {
						job = label.Value
					}
				}
				if job != "bar" || (name != alertMetricName && name != alertForStateMetricName) || seen[name] {
					t.Fatalf("unexpected or duplicate resolve series: %+v", series)
				}
				seen[name] = true
				if len(series.Samples) != 1 || !decimal.IsStaleNaN(series.Samples[0].Value) || series.Samples[0].Timestamp != ts.Unix()*1000 {
					t.Fatalf("expected a stale sample at the evaluation timestamp: %+v", series)
				}
			}
			if ar.alerts[id].State != notifier.StateInactive {
				t.Fatal("expected retained alert to be inactive")
			}
		})
	}
}

func TestLimitResolveRetriedUntilAcknowledged(t *testing.T) {
	ar, _, id := newLimitResolveRule(t)
	ts := time.Now()
	lastSent := ar.alerts[id].LastSent
	for _, offset := range []time.Duration{0, time.Minute, 16 * time.Minute} {
		_, err := ar.exec(context.Background(), ts.Add(offset), 1, nil)
		if !errors.Is(err, errExecLimitExceeded) {
			t.Fatalf("expected limit error; got %v", err)
		}
		if len(ar.alerts) != 1 || ar.alerts[id] == nil || ar.alerts[id].State != notifier.StateInactive {
			t.Fatalf("resolve was lost on retry: %+v", ar.alerts)
		}
		if !ar.alerts[id].ResolvedAt.Equal(ts) {
			t.Fatal("retry changed the original resolution timestamp")
		}
		alerts := ar.alertsToSend(time.Hour, time.Hour)
		if len(alerts) != 1 || alerts[0].ID != id || !alerts[0].End.Equal(ts) {
			t.Fatalf("resolve was suppressed by resend delay: %+v", alerts)
		}
		if !ar.alerts[id].LastSent.Equal(lastSent) {
			t.Fatal("undelivered resolve changed LastSent")
		}
	}
	ar.ackLimitResolves(nil)
	if len(ar.alerts) != 1 {
		t.Fatal("unselected resolve was purged")
	}
	ar.ackLimitResolves(ar.alertsToSend(time.Hour, time.Hour))
	if len(ar.alerts) != 0 || len(ar.limitResolves) != 0 {
		t.Fatal("delivered resolve was not removed")
	}
}

func TestLimitResolveSurvivesRecovery(t *testing.T) {
	ar, fq, id := newLimitResolveRule(t)
	ts := time.Now()
	if _, err := ar.exec(context.Background(), ts, 1, nil); !errors.Is(err, errExecLimitExceeded) {
		t.Fatal(err)
	}
	fq.Reset()
	fq.Add(metricWithValueAndLabels(t, 1, "job", "qux"))
	if _, err := ar.exec(context.Background(), ts.Add(time.Minute), 1, nil); err != nil {
		t.Fatal(err)
	}
	alerts := ar.alertsToSend(time.Hour, time.Hour)
	if len(alerts) != 2 {
		t.Fatalf("want a resolve and the new firing alert; got %+v", alerts)
	}
	ar.ackLimitResolves(alerts)
	if len(ar.alerts) != 1 || ar.alerts[id] != nil {
		t.Fatal("acknowledgement dropped the new firing alert or retained the resolve")
	}
}

func TestExecutorLimitResolveDelivery(t *testing.T) {
	oldResendDelay := *resendDelay
	*resendDelay = time.Hour
	t.Cleanup(func() { *resendDelay = oldResendDelay })
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var alerts []struct {
			EndsAt time.Time `json:"endsAt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&alerts); err != nil {
			t.Error(err)
			w.WriteHeader(500)
			return
		}
		if len(alerts) != 1 || alerts[0].EndsAt.IsZero() {
			t.Errorf("expected one resolve; got %+v", alerts)
		}
		if attempts.Add(1) < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()
	initLimitNotifierForTest(t, srv.URL)
	ar, _, id := newLimitResolveRule(t)
	ts := time.Now()
	e := &executor{}
	for i := 0; i < 3; i++ {
		err := e.exec(context.Background(), ar, ts.Add(time.Duration(i)*time.Minute), time.Hour, 1, nil)
		if !errors.Is(err, errExecLimitExceeded) {
			t.Fatalf("expected recorded limit error; got %v", err)
		}
		if got := attempts.Load(); got != int32(i+1) {
			t.Fatalf("got %d delivery attempts; want %d", got, i+1)
		}
		if i < 2 && ar.alerts[id] == nil {
			t.Fatal("failed resolve was dropped")
		}
	}
	if len(ar.alerts) != 0 || len(ar.limitResolves) != 0 {
		t.Fatal("successful resolves were not cleared")
	}
}

func newLimitResolveRule(t *testing.T) (*AlertingRule, *datasource.FakeQuerier, uint64) {
	t.Helper()
	fq := &datasource.FakeQuerier{}
	fq.Add(metricWithValueAndLabels(t, 1, "job", "bar"), metricWithValueAndLabels(t, 1, "job", "baz"))
	ar := newTestAlertingRule("test", 0)
	ar.q = fq
	labels := map[string]string{alertNameLabel: "test", "job": "bar"}
	id := hash(labels)
	ar.alerts[id] = &notifier.Alert{ID: id, Name: "test", Labels: labels, State: notifier.StateFiring, LastSent: time.Now().Add(time.Hour)}
	return ar, fq, id
}

func initLimitNotifierForTest(t *testing.T, addr string) {
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
	targets := notifier.GetTargets()
	t.Cleanup(func() {
		for _, group := range targets {
			for _, target := range group {
				target.Close()
			}
		}
	})
}
