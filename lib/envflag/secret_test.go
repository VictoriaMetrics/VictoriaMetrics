package envflag

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

func TestApplySecretFlags(t *testing.T) {
	t.Cleanup(flagutil.UnregisterAllSecretFlags)
	secretFlagsList = &flagutil.ArrayString{}
	if err := secretFlagsList.Set("foo,bar"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if flagutil.IsSecretFlag("foo") || flagutil.IsSecretFlag("bar") {
		t.Fatalf("foo and bar shouldn't be secret before applySecretFlags")
	}

	applySecretFlags()

	if !flagutil.IsSecretFlag("foo") || !flagutil.IsSecretFlag("bar") {
		t.Fatalf("foo and bar should be secret after applySecretFlags")
	}
}
