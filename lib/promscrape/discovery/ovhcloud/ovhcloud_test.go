package ovhcloud

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func Test_GetLabels(t *testing.T) {
	t.Run("incorrect service", func(t *testing.T) {
		sdc := &SDConfig{
			Endpoint:          "ovh-ca",
			ApplicationKey:    "no-op",
			ApplicationSecret: &promauth.Secret{S: "no-op"},
			ConsumerKey:       &promauth.Secret{S: "no-op"},
			Service:           "incorrect service",
		}
		if _, err := sdc.GetLabels(""); err == nil {
			t.Fatalf("newAPIConfig want err, got: %v", err)
		}
	})
}
