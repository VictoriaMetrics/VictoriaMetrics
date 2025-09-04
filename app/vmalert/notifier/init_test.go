package notifier

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

func TestInit(t *testing.T) {
	oldAddrs := *addrs
	defer func() { *addrs = oldAddrs }()

	*addrs = flagutil.ArrayString{"127.0.0.1", "127.0.0.2"}

	fn, err := Init(nil, "")
	if err != nil {
		t.Fatalf("%s", err)
	}

	nfs := fn()
	if len(nfs) != 2 {
		t.Fatalf("expected to get 2 notifiers; got %d", len(nfs))
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
		if _, err := Init(nil, ""); err == nil {
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

	fn, err := Init(nil, "")
	if err != nil {
		t.Fatalf("%s", err)
	}

	nfs := fn()
	if len(nfs) != 1 {
		t.Fatalf("expected to get 1 notifier; got %d", len(nfs))
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
