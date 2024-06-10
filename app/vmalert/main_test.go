package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

func init() {
	// Disable rand sleep on group start during tests in order to speed up test execution.
	// Rand sleep is needed only in prod code.
	rule.SkipRandSleepOnGroupStart = true
}

func TestGetExternalURL(t *testing.T) {
	invalidURL := "victoriametrics.com/path"
	_, err := getExternalURL(invalidURL)
	if err == nil {
		t.Errorf("expected error, got nil")
	}

	expURL := "https://victoriametrics.com/path"
	u, err := getExternalURL(expURL)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if u.String() != expURL {
		t.Errorf("unexpected url: want %q, got %s", expURL, u.String())
	}

	h, _ := os.Hostname()
	expURL = fmt.Sprintf("http://%s:8880", h)
	u, err = getExternalURL("")
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if u.String() != expURL {
		t.Errorf("unexpected url: want %s, got %s", expURL, u.String())
	}
}

func TestGetAlertURLGenerator(t *testing.T) {
	testAlert := notifier.Alert{GroupID: 42, ID: 2, Value: 4, Labels: map[string]string{"tenant": "baz"}}
	u, _ := url.Parse("https://victoriametrics.com/path")
	fn, err := getAlertURLGenerator(u, "", false)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	exp := fmt.Sprintf("https://victoriametrics.com/path/vmalert/alert?%s=42&%s=2", paramGroupID, paramAlertID)
	if exp != fn(testAlert) {
		t.Errorf("unexpected url want %s, got %s", exp, fn(testAlert))
	}
	_, err = getAlertURLGenerator(nil, "foo?{{invalid}}", true)
	if err == nil {
		t.Errorf("expected template validation error got nil")
	}
	fn, err = getAlertURLGenerator(u, "foo?query={{$value}}&ds={{ $labels.tenant }}", true)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if exp := "https://victoriametrics.com/path/foo?query=4&ds=baz"; exp != fn(testAlert) {
		t.Errorf("unexpected url want %s, got %s", exp, fn(testAlert))
	}
}

func TestConfigReload(t *testing.T) {
	originalRulePath := *rulePath
	defer func() {
		*rulePath = originalRulePath
	}()

	const (
		rules1 = `
groups:
  - name: group-1
    rules:
      - alert: ExampleAlertAlwaysFiring
        expr: sum by(job) (up == 1)
      - record: handler:requests:rate5m 
        expr: sum(rate(prometheus_http_requests_total[5m])) by (handler)
`
		rules2 = `
groups:
  - name: group-1
    rules:
      - alert: ExampleAlertAlwaysFiring
        expr: sum by(job) (up == 1)
  - name: group-2
    rules:
      - record: handler:requests:rate5m 
        expr: sum(rate(prometheus_http_requests_total[5m])) by (handler)
`
	)

	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	writeToFile(t, f.Name(), rules1)

	*configCheckInterval = 200 * time.Millisecond
	*rulePath = []string{f.Name()}
	ctx, cancel := context.WithCancel(context.Background())

	m := &manager{
		querierBuilder: &datasource.FakeQuerier{},
		groups:         make(map[uint64]*rule.Group),
		labels:         map[string]string{},
		notifiers:      func() []notifier.Notifier { return []notifier.Notifier{&notifier.FakeNotifier{}} },
		rw:             &remotewrite.Client{},
	}

	syncCh := make(chan struct{})
	sighupCh := procutil.NewSighupChan()
	go func() {
		configReload(ctx, m, nil, sighupCh)
		close(syncCh)
	}()

	lenLocked := func(m *manager) int {
		m.groupsMu.RLock()
		defer m.groupsMu.RUnlock()
		return len(m.groups)
	}

	checkCfg := func(err error) {
		cErr := getLastConfigError()
		cfgSuc := configSuccess.Get()
		if err != nil {
			if cErr == nil {
				t.Fatalf("expected to have config error %s; got nil instead", cErr)
			}
			if cfgSuc != 0 {
				t.Fatalf("expected to have metric configSuccess to be set to 0; got %v instead", cfgSuc)
			}
			return
		}

		if cErr != nil {
			t.Fatalf("unexpected config error: %s", cErr)
		}
		if cfgSuc != 1 {
			t.Fatalf("expected to have metric configSuccess to be set to 1; got %v instead", cfgSuc)
		}
	}

	time.Sleep(*configCheckInterval * 2)
	checkCfg(nil)
	groupsLen := lenLocked(m)
	if groupsLen != 1 {
		t.Fatalf("expected to have exactly 1 group loaded; got %d", groupsLen)
	}

	writeToFile(t, f.Name(), rules2)
	time.Sleep(*configCheckInterval * 2)
	checkCfg(nil)
	groupsLen = lenLocked(m)
	if groupsLen != 2 {
		fmt.Println(m.groups)
		t.Fatalf("expected to have exactly 2 groups loaded; got %d", groupsLen)
	}

	writeToFile(t, f.Name(), rules1)
	procutil.SelfSIGHUP()
	time.Sleep(*configCheckInterval / 2)
	checkCfg(nil)
	groupsLen = lenLocked(m)
	if groupsLen != 1 {
		t.Fatalf("expected to have exactly 1 group loaded; got %d", groupsLen)
	}

	writeToFile(t, f.Name(), `corrupted`)
	procutil.SelfSIGHUP()
	time.Sleep(*configCheckInterval / 2)
	checkCfg(fmt.Errorf("config error"))
	groupsLen = lenLocked(m)
	if groupsLen != 1 { // should remain unchanged
		t.Fatalf("expected to have exactly 1 group loaded; got %d", groupsLen)
	}

	cancel()
	<-syncCh
}

func writeToFile(t *testing.T, file, b string) {
	t.Helper()
	err := os.WriteFile(file, []byte(b), 0644)
	if err != nil {
		t.Fatal(err)
	}
}
