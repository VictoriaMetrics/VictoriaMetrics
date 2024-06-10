package kubernetes

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func TestParseKubeConfigSuccess(t *testing.T) {
	f := func(configFile string, expectedConfig *kubeConfig) {
		t.Helper()

		config, err := newKubeConfig(configFile)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !reflect.DeepEqual(config, expectedConfig) {
			t.Fatalf("unexpected result, got: %v, want: %v", config, expectedConfig)
		}
	}

	f("testdata/good_kubeconfig/with_token.yaml", &kubeConfig{
		server: "http://some-server:8080",
		token:  "abc",
	})

	f("testdata/good_kubeconfig/with_tls.yaml", &kubeConfig{
		server: "https://localhost:6443",
		tlsConfig: &promauth.TLSConfig{
			CA:   "authority",
			Cert: "certificate",
			Key:  "key",
		},
	})

	f("testdata/good_kubeconfig/with_basic.yaml", &kubeConfig{
		server: "http://some-server:8080",
		basicAuth: &promauth.BasicAuthConfig{
			Password: promauth.NewSecret("secret"),
			Username: "user1",
		},
	})
}

func TestParseKubeConfigFail(t *testing.T) {
	f := func(name, kubeConfigFile string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if _, err := newKubeConfig(kubeConfigFile); err == nil {
				t.Fatalf("unexpected result for config file: %s, must return error", kubeConfigFile)
			}
		})
	}
	f("unsupported options", "testdata/bad_kubeconfig/unsupported_fields")
	f("missing server address", "testdata/bad_kubeconfig/missing_server.yaml")
}
