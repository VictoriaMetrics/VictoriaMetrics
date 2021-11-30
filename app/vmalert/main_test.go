package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
)

func TestGetExternalURL(t *testing.T) {
	expURL := "https://vicotriametrics.com/path"
	u, err := getExternalURL(expURL, "", false)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if u.String() != expURL {
		t.Errorf("unexpected url want %s, got %s", expURL, u.String())
	}
	h, _ := os.Hostname()
	expURL = fmt.Sprintf("https://%s:4242", h)
	u, err = getExternalURL("", "0.0.0.0:4242", true)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if u.String() != expURL {
		t.Errorf("unexpected url want %s, got %s", expURL, u.String())
	}
}

func TestGetAlertURLGenerator(t *testing.T) {
	testAlert := notifier.Alert{GroupID: 42, ID: 2, Value: 4}
	u, _ := url.Parse("https://victoriametrics.com/path")
	fn, err := getAlertURLGenerator(u, "", false)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if exp := "https://victoriametrics.com/path/api/v1/42/2/status"; exp != fn(testAlert) {
		t.Errorf("unexpected url want %s, got %s", exp, fn(testAlert))
	}
	_, err = getAlertURLGenerator(nil, "foo?{{invalid}}", true)
	if err == nil {
		t.Errorf("expected tempalte validation error got nil")
	}
	fn, err = getAlertURLGenerator(u, "foo?query={{$value}}", true)
	if err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if exp := "https://victoriametrics.com/path/foo?query=4"; exp != fn(testAlert) {
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

	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Fatal(err)
	}
	writeToFile(t, f.Name(), rules1)

	*rulesCheckInterval = 200 * time.Millisecond
	*rulePath = []string{f.Name()}
	ctx, cancel := context.WithCancel(context.Background())

	m := &manager{
		querierBuilder: &fakeQuerier{},
		groups:         make(map[uint64]*Group),
		labels:         map[string]string{},
		notifiers:      []notifier.Notifier{&fakeNotifier{}},
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

	time.Sleep(*rulesCheckInterval * 2)
	groupsLen := lenLocked(m)
	if groupsLen != 1 {
		t.Fatalf("expected to have exactly 1 group loaded; got %d", groupsLen)
	}

	writeToFile(t, f.Name(), rules2)
	time.Sleep(*rulesCheckInterval * 2)
	groupsLen = lenLocked(m)
	if groupsLen != 2 {
		fmt.Println(m.groups)
		t.Fatalf("expected to have exactly 2 groups loaded; got %d", groupsLen)
	}

	writeToFile(t, f.Name(), rules1)
	procutil.SelfSIGHUP()
	time.Sleep(*rulesCheckInterval / 2)
	groupsLen = lenLocked(m)
	if groupsLen != 1 {
		t.Fatalf("expected to have exactly 1 group loaded; got %d", groupsLen)
	}

	writeToFile(t, f.Name(), `corrupted`)
	procutil.SelfSIGHUP()
	time.Sleep(*rulesCheckInterval / 2)
	groupsLen = lenLocked(m)
	if groupsLen != 1 { // should remain unchanged
		t.Fatalf("expected to have exactly 1 group loaded; got %d", groupsLen)
	}

	cancel()
	<-syncCh
}

func writeToFile(t *testing.T, file, b string) {
	t.Helper()
	err := ioutil.WriteFile(file, []byte(b), 0644)
	if err != nil {
		t.Fatal(err)
	}
}
