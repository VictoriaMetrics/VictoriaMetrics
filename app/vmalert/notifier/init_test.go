package notifier

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

func TestInit(t *testing.T) {
	oldAddrs := *addrs
	defer func() { *addrs = oldAddrs }()

	*addrs = flagutil.ArrayString{"127.0.0.1", "127.0.0.2"}

	fn, err := Init(nil, nil, "")
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

func TestInitBlackHole(t *testing.T) {
	oldBlackHole := *blackHole
	defer func() { *blackHole = oldBlackHole }()

	*blackHole = true

	fn, err := Init(nil, nil, "")
	if err != nil {
		t.Fatalf("%s", err)
	}

	nfs := fn()
	if len(nfs) != 1 {
		t.Fatalf("expected to get 1 notifiers; got %d", len(nfs))
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

func TestInitBlackHoleWithNotifierUrl(t *testing.T) {
	oldAddrs := *addrs
	oldBlackHole := *blackHole
	defer func() {
		*addrs = oldAddrs
		*blackHole = oldBlackHole
	}()

	*addrs = flagutil.ArrayString{"127.0.0.1", "127.0.0.2"}
	*blackHole = true

	_, err := Init(nil, nil, "")
	if err == nil {
		t.Fatalf("Expect Init to return error; instead got no error")
	}

}

func TestInitBlackHoleWithNotifierConfig(t *testing.T) {
	oldConfigPath := *configPath
	oldBlackHole := *blackHole
	defer func() {
		*configPath = oldConfigPath
		*blackHole = oldBlackHole
	}()

	*configPath = "/dummy/path"
	*blackHole = true

	fn, err := Init(nil, nil, "")
	if err == nil {
		t.Fatalf("Expect Init to return error; instead got no error")
	}

	if fn != nil {
		t.Fatalf("expected no notifiers to be returned;but got %v instead", fn())
	}
}
