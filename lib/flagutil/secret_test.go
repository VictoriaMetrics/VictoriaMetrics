package flagutil

import "testing"

func TestApplySecretFlags(t *testing.T) {
	secretFlags = make(map[string]bool)
	secretFlagsList = &ArrayString{}
	if err := secretFlagsList.Set("foo,bar"); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	if IsSecretFlag("foo") || IsSecretFlag("bar") {
		t.Fatalf("flags are secret before ApplySecretFlags")
	}

	ApplySecretFlags()

	if !IsSecretFlag("foo") || !IsSecretFlag("bar") {
		t.Fatalf("ApplySecretFlags didn't mark flags as secret")
	}
}
