package ovhcloud

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func Test_newAPIConfig(t *testing.T) {
	t.Run("normal case", func(t *testing.T) {
		sdc := &SDConfig{
			Endpoint:          "ovh-ca",
			ApplicationKey:    "no-op",
			ApplicationSecret: &promauth.Secret{S: "no-op"},
			ConsumerKey:       &promauth.Secret{S: "no-op"},
			Service:           "vps",
		}
		if _, err := newAPIConfig(sdc, ""); err != nil {
			t.Fatalf("newAPIConfig got error: %v", err)
		}
	})

	t.Run("incorrect endpoint", func(t *testing.T) {
		sdc := &SDConfig{
			Endpoint:          "in-correct-endpoint",
			ApplicationKey:    "no-op",
			ApplicationSecret: &promauth.Secret{S: "no-op"},
			ConsumerKey:       &promauth.Secret{S: "no-op"},
			Service:           "vps",
		}
		if _, err := newAPIConfig(sdc, ""); err == nil {
			t.Fatalf("newAPIConfig want error, but error = %v", err)
		}
	})
}
