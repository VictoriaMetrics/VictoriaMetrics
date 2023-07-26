package notifier

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"testing"
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
