package flagutil

import "testing"

func TestApplySecretFlags(t *testing.T) {
	secretFlags = make(map[string]bool)
	secretFlagsList = &ArrayString{}
	secretFlagsList.Set("foo,bar")

	if IsSecretFlag("foo") || IsSecretFlag("bar") {
		t.Fatalf("flags are secret before ApplySecretFlags")
	}

	ApplySecretFlags()

	if !IsSecretFlag("foo") || !IsSecretFlag("bar") {
		t.Fatalf("ApplySecretFlags didn't mark flags as secret")
	}
}
