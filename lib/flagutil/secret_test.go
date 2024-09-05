package flagutil

import "testing"

func TestApplySecretFlags(t *testing.T) {
	t.Cleanup(func() {
		secretFlags = make(map[string]bool)
	})
	secretFlagsList = &ArrayString{}
	if err := secretFlagsList.Set("foo,bar"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if IsSecretFlag("foo") || IsSecretFlag("bar") {
		t.Fatalf("foo and bar shouldn't be secret before ApplySecretFlags")
	}

	ApplySecretFlags()

	if !IsSecretFlag("foo") || !IsSecretFlag("bar") {
		t.Fatalf("foo and bar should be secret after ApplySecretFlags")
	}
}
