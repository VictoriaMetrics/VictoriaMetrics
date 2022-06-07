package kubernetes

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func TestParseKubeConfigSuccess(t *testing.T) {

	type testCase struct {
		name           string
		kubeConfigFile string
		expectedConfig *kubeConfig
	}

	var cases = []testCase{
		{
			name:           "token",
			kubeConfigFile: "testdata/good_kubeconfig/with_token.yaml",
			expectedConfig: &kubeConfig{
				server: "http://some-server:8080",
				token:  "abc",
			},
		},
		{
			name:           "cert",
			kubeConfigFile: "testdata/good_kubeconfig/with_tls.yaml",
			expectedConfig: &kubeConfig{
				server: "https://localhost:6443",
				tlsConfig: &promauth.TLSConfig{
					CA:   []byte("authority"),
					Cert: []byte("certificate"),
					Key:  []byte("key"),
				},
			},
		},
		{
			name:           "basic",
			kubeConfigFile: "testdata/good_kubeconfig/with_basic.yaml",
			expectedConfig: &kubeConfig{
				server: "http://some-server:8080",
				basicAuth: &promauth.BasicAuthConfig{
					Password: promauth.NewSecret("secret"),
					Username: "user1",
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ac, err := newKubeConfig(tc.kubeConfigFile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(ac, tc.expectedConfig) {
				t.Fatalf("unexpected result, got: %v, want: %v", ac, tc.expectedConfig)
			}
		})
	}
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
