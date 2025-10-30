package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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

	f := func(path, addr string, bh bool) {
		*configPath = path
		*addrs = flagutil.ArrayString{addr}
		*blackHole = bh
		if err := Init(nil, ""); err == nil {
			t.Fatalf("expected to get error; got nil instead")
		}
	}

	// *configPath, *addrs and *blackhole are mutually exclusive
	f("/dummy/path", "127.0.0.1", false)
	f("/dummy/path", "", true)
	f("", "127.0.0.1", true)
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
	if errG.Err() != nil {
		t.Fatalf("unexpected error when sending alerts: %s", err)
	}
}
