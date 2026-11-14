package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestThanosStateParamsIdentityIncludesPrecision(t *testing.T) {
	tests := []struct {
		name   string
		modify func(*thanosStateParams)
	}{
		{
			name: "significant figures",
			modify: func(p *thanosStateParams) {
				p.SignificantFigures = 3
			},
		},
		{
			name: "round digits",
			modify: func(p *thanosStateParams) {
				p.RoundDigits = 4
			},
		},
	}

	base := thanosStateParams{
		Snapshot:           "/tmp/thanos-snapshot",
		DstAddr:            "http://localhost:8428",
		AccountID:          "0",
		ExtraLabels:        []string{"env=test"},
		SignificantFigures: 5,
		RoundDigits:        2,
	}

	baseIdentity := stateParamsIdentity(t, base)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changed := base
			tt.modify(&changed)

			changedIdentity := stateParamsIdentity(t, changed)

			if baseIdentity == changedIdentity {
				t.Fatalf(
					"expected migration identity to change when %s changes",
					tt.name,
				)
			}
		})
	}
}

func TestThanosStateParamsIdentityStableForSamePrecision(t *testing.T) {
	params := thanosStateParams{
		Snapshot:           "/tmp/thanos-snapshot",
		DstAddr:            "http://localhost:8428",
		AccountID:          "0",
		ExtraLabels:        []string{"env=test"},
		SignificantFigures: 5,
		RoundDigits:        2,
	}

	first := stateParamsIdentity(t, params)
	second := stateParamsIdentity(t, params)

	if first != second {
		t.Fatalf(
			"expected identical migration parameters to produce the same identity; got %q and %q",
			first,
			second,
		)
	}
}

func TestThanosStateParamsIdentityDistinguishesDefaultPrecision(t *testing.T) {
	defaults := thanosStateParams{
		Snapshot: "/tmp/thanos-snapshot",
		DstAddr:  "http://localhost:8428",
	}

	explicit := defaults
	explicit.SignificantFigures = 0
	explicit.RoundDigits = 100

	if stateParamsIdentity(t, defaults) == stateParamsIdentity(t, explicit) {
		t.Fatal("expected different precision configurations to produce different identities")
	}
}

func stateParamsIdentity(t *testing.T, params thanosStateParams) string {
	t.Helper()

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("failed to marshal state parameters: %v", err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:8])
}
