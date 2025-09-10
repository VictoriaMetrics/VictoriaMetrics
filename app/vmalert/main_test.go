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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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
		t.Fatalf("expected error, got nil")
	}

	expURL := "https://victoriametrics.com/path"
	u, err := getExternalURL(expURL)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if u.String() != expURL {
		t.Fatalf("unexpected url: want %q, got %s", expURL, u.String())
	}

	h, _ := os.Hostname()
	expURL = fmt.Sprintf("http://%s:8880", h)
	u, err = getExternalURL("")
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if u.String() != expURL {
		t.Fatalf("unexpected url: want %s, got %s", expURL, u.String())
	}
}

func TestConfigReload(t *testing.T) {
	originalRulePath := *rulePath
	originalExternalURL := extURL
	extURL = &url.URL{}
	defer func() {
		extURL = originalExternalURL
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
	defer fs.MustRemovePath(f.Name())
	writeToFile(f.Name(), rules1)

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

	writeToFile(f.Name(), rules2)
	time.Sleep(*configCheckInterval * 2)
	checkCfg(nil)
	groupsLen = lenLocked(m)
	if groupsLen != 2 {
		t.Fatalf("expected to have exactly 2 groups loaded; got %d", groupsLen)
	}

	writeToFile(f.Name(), rules1)
	procutil.SelfSIGHUP()
	time.Sleep(*configCheckInterval / 2)
	checkCfg(nil)
	groupsLen = lenLocked(m)
	if groupsLen != 1 {
		t.Fatalf("expected to have exactly 1 group loaded; got %d", groupsLen)
	}

	writeToFile(f.Name(), `corrupted`)
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

func writeToFile(file, b string) {
	fs.MustWriteSync(file, []byte(b))
}
