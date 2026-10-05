package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestInit(t *testing.T) {
	oldAddrs := *addrs
	defer func() { *addrs = oldAddrs }()

	*addrs = flagutil.ArrayString{"127.0.0.1", "127.0.0.2"}

	err := Init(nil, "")
	if err != nil {
		t.Fatalf("%s", err)
	}

	if len(getActiveNotifiers()) != 2 {
		t.Fatalf("expected to get 2 notifiers; got %d", len(getActiveNotifiers()))
	}

	targets := GetTargets()
	if targets == nil || targets[TargetStatic] == nil {
		t.Fatalf("expected to get static targets in response")
	}

	nf1 := targets[TargetStatic][0]
	if nf1.Addr() != "127.0.0.1/api/v2/alerts" {
		t.Fatalf("expected to get \"127.0.0.1/api/v2/alerts\"; got %q instead", nf1.Addr())
	}
	nf2 := targets[TargetStatic][1]
	if nf2.Addr() != "127.0.0.2/api/v2/alerts" {
		t.Fatalf("expected to get \"127.0.0.2/api/v2/alerts\"; got %q instead", nf2.Addr())
	}
}

func TestInitNegative(t *testing.T) {
	oldConfigPath := *configPath
	oldAddrs := *addrs
	oldBlackHole := *blackHole

	defer func() {
		*configPath = oldConfigPath
		*addrs = oldAddrs
		*blackHole = oldBlackHole
	}()

	f := func(path string, addr []string, bh bool) {
		*configPath = path
		*addrs = flagutil.ArrayString(addr)
		*blackHole = bh
		if err := Init(nil, ""); err == nil {
			t.Fatalf("expected to get error; got nil instead")
		}
	}

	// *configPath, *addrs and *blackhole are mutually exclusive
	f("/dummy/path", []string{"127.0.0.1"}, false)
	f("/dummy/path", []string{}, true)
	f("", []string{"127.0.0.1"}, true)
	// addr cannot be ""
	f("", []string{""}, false)
	f("", []string{"127.0.0.1", ""}, false)
}

func TestBlackHole(t *testing.T) {
	oldBlackHole := *blackHole
	defer func() { *blackHole = oldBlackHole }()

	*blackHole = true

	err := Init(nil, "")
	if err != nil {
		t.Fatalf("%s", err)
	}

	if len(getActiveNotifiers()) != 1 {
		t.Fatalf("expected to get 1 notifier; got %d", len(getActiveNotifiers()))
	}

	targets := GetTargets()
	if targets == nil || targets[TargetStatic] == nil {
		t.Fatalf("expected to get static targets in response")
	}
	if len(targets[TargetStatic]) != 1 {
		t.Fatalf("expected to get 1 static targets in response; but got %d", len(targets[TargetStatic]))
	}
	nf1 := targets[TargetStatic][0]
	if nf1.Addr() != "blackhole" {
		t.Fatalf("expected to get \"blackhole\"; got %q instead", nf1.Addr())
	}
}

func TestGetAlertURLGenerator(t *testing.T) {
	oldAlertURLGeneratorFn := AlertURLGeneratorFn
	defer func() { AlertURLGeneratorFn = oldAlertURLGeneratorFn }()

	testAlert := Alert{GroupID: 42, ID: 2, Value: 4, Labels: map[string]string{"tenant": "baz"}}
	u, _ := url.Parse("https://victoriametrics.com/path")
	err := InitAlertURLGeneratorFn(u, "", false)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	exp := fmt.Sprintf("https://victoriametrics.com/path/vmalert/alert?%s=42&%s=2", "group_id", "alert_id")
	if exp != AlertURLGeneratorFn(testAlert) {
		t.Fatalf("unexpected url want %s, got %s", exp, AlertURLGeneratorFn(testAlert))
	}
	err = InitAlertURLGeneratorFn(nil, "foo?{{invalid}}", true)
	if err == nil {
		t.Fatalf("expected template validation error got nil")
	}
	err = InitAlertURLGeneratorFn(u, "foo?query={{$value}}&ds={{ $labels.tenant }}", true)
	if err != nil {
		t.Fatalf("unexpected error %s", err)
	}
	if exp := "https://victoriametrics.com/path/foo?query=4&ds=baz"; exp != AlertURLGeneratorFn(testAlert) {
		t.Fatalf("unexpected url want %s, got %s", exp, AlertURLGeneratorFn(testAlert))
	}
}

func TestSendAlerts(t *testing.T) {
	oldAlertURLGeneratorFn := AlertURLGeneratorFn
	defer func() { AlertURLGeneratorFn = oldAlertURLGeneratorFn }()
	AlertURLGeneratorFn = func(alert Alert) string {
		return ""
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatalf("should not be called")
	})
	mux.HandleFunc(alertManagerPath, func(w http.ResponseWriter, r *http.Request) {
		var a []struct {
			Labels map[string]string `json:"labels"`
		}
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			t.Fatalf("can not unmarshal data into alert %s", err)
		}
		if len(a) != 2 {
			t.Fatalf("expected 2 alert in array got %d", len(a))
		}
		if len(a[0].Labels) != 4 {
			t.Fatalf("expected 4 labels got %d", len(a[0].Labels))
		}
		if a[0].Labels["env"] != "prod" {
			t.Fatalf("expected env label to be prod during relabeling, got %s", a[0].Labels["env"])
		}
		if a[0].Labels["c"] != "baz" {
			t.Fatalf("expected c label to be baz during relabeling, got %s", a[0].Labels["c"])
		}
		if len(a[1].Labels) != 1 {
			t.Fatalf("expected 1 labels got %d", len(a[1].Labels))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer fs.MustRemovePath(f.Name())

	rawConfig := `
static_configs:
  - targets:
      - %s
    alert_relabel_configs:
    - source_labels: [b]
      target_label: "c"
alert_relabel_configs:
  - source_labels: [a]
    target_label: "b"
  - target_label: "env"
    replacement: "prod"
`
	config := fmt.Sprintf(rawConfig, srv.URL+alertManagerPath)
	writeToFile(f.Name(), config)

	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()
	*configPath = f.Name()
	err = Init(nil, "")
	if err != nil {
		t.Fatalf("unexpected error when parse notifier config: %s", err)
	}

	firingAlerts := []Alert{
		{
			Name:   "alert1",
			Labels: map[string]string{"a": "baz"},
		},
		{
			Name:   "alert2",
			Labels: map[string]string{},
		},
	}
	errG := Send(context.Background(), firingAlerts, nil)
	for err := range errG {
		if err != nil {
			t.Errorf("unexpected error when sending alerts: %s", err)
		}
	}
}

// TestSendAlertsPerNotifierRelabel reproduces
// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11676:
// Send fans the same relabeled label slices out to every active notifier,
// and every notifier applies its own alert_relabel_configs. That relabeling
// must not mutate the shared slice, otherwise concurrently sending notifiers
// observe each other's partially applied relabel chains. The chain is the
// mark-then-append idiom: owner=a alerts get team-a and are marked, unmarked
// alerts get team-b, the mark is dropped and the leading separator trimmed.
func TestSendAlertsPerNotifierRelabel(t *testing.T) {
	oldAlertURLGeneratorFn := AlertURLGeneratorFn
	defer func() { AlertURLGeneratorFn = oldAlertURLGeneratorFn }()
	AlertURLGeneratorFn = func(_ Alert) string {
		return ""
	}

	var mu sync.Mutex
	var corrupted []string
	mux := http.NewServeMux()
	mux.HandleFunc(alertManagerPath, func(_ http.ResponseWriter, r *http.Request) {
		var a []struct {
			Labels map[string]string `json:"labels"`
		}
		if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
			t.Errorf("can not unmarshal data into alert %s", err)
			return
		}
		for _, alert := range a {
			// owner=a alerts are claimed by the first rule and must skip the
			// team-b append; owner=b alerts must take it. Either way the
			// temporary mark is dropped and the leading separator trimmed.
			want := "team-b"
			if alert.Labels["owner"] == "a" {
				want = "team-a"
			}
			if alert.Labels["teams"] != want || alert.Labels["__tmp_owned"] != "" {
				mu.Lock()
				corrupted = append(corrupted, fmt.Sprintf("%v", alert.Labels))
				mu.Unlock()
			}
		}
	})
	const notifiersCount = 3
	targets := make([]string, 0, notifiersCount)
	for i := 0; i < notifiersCount; i++ {
		srv := httptest.NewServer(mux)
		defer srv.Close()
		targets = append(targets, srv.URL+alertManagerPath)
	}

	f, err := os.CreateTemp("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer fs.MustRemovePath(f.Name())

	rawConfig := `
static_configs:
  - targets:
      - %s
      - %s
      - %s
    alert_relabel_configs:
    - source_labels: [teams, owner]
      separator: ";"
      regex: "(.*);a"
      target_label: teams
      replacement: "${1},team-a"
    - source_labels: [teams]
      regex: ".*team-a.*"
      target_label: __tmp_owned
      replacement: "1"
    - source_labels: [teams, __tmp_owned]
      separator: ";"
      regex: "(.*);"
      target_label: teams
      replacement: "${1},team-b"
    - regex: __tmp_owned
      action: labeldrop
    - source_labels: [teams]
      regex: ",(.*)"
      target_label: teams
      replacement: "${1}"
`
	config := fmt.Sprintf(rawConfig, targets[0], targets[1], targets[2])
	writeToFile(f.Name(), config)

	oldConfigPath := configPath
	defer func() { configPath = oldConfigPath }()
	*configPath = f.Name()
	if err := Init(nil, ""); err != nil {
		t.Fatalf("unexpected error when parse notifier config: %s", err)
	}
	if n := len(getActiveNotifiers()); n != notifiersCount {
		t.Fatalf("expected %d notifiers, got %d", notifiersCount, n)
	}

	alerts := make([]Alert, 0, 300)
	for i := 0; i < 300; i++ {
		owner := "a"
		if i%2 == 1 {
			owner = "b"
		}
		alerts = append(alerts, Alert{
			Name:   "test",
			Labels: map[string]string{"alertname": "test", "id": fmt.Sprintf("%d", i), "owner": owner},
		})
	}
	for round := 0; round < 20; round++ {
		for err := range Send(context.Background(), alerts, nil) {
			if err != nil {
				t.Fatalf("unexpected error when sending alerts: %s", err)
			}
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(corrupted) > 0 {
		t.Fatalf("%d alerts reached notifiers with a partially relabeled label set; first: %s", len(corrupted), corrupted[0])
	}
}
