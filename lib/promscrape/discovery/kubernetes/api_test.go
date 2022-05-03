package kubernetes

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"reflect"
	"testing"
)

func TestParseKubeConfig(t *testing.T) {

	type testCase struct {
		name           string
		sdc            *SDConfig
		expectedConfig *ApiConfig
	}

	var cases = []testCase{
		{
			name: "token",
			sdc: &SDConfig{
				KubeConfig: "testdata/kubeconfig_token.yaml",
			},
			expectedConfig: &ApiConfig{
				token:     "abc",
				tlsConfig: &promauth.TLSConfig{},
			},
		},
		{
			name: "cert",
			sdc: &SDConfig{
				KubeConfig: "testdata/kubeconfig_cert.yaml",
			},
			expectedConfig: &ApiConfig{
				server: "localhost:8000",
				tlsConfig: &promauth.TLSConfig{
					CA:   []byte("authority"),
					Cert: []byte("certificate"),
					Key:  []byte("key"),
				},
			},
		},
		{
			name: "basic",
			sdc: &SDConfig{
				KubeConfig: "testdata/kubeconfig_basic.yaml",
			},
			expectedConfig: &ApiConfig{
				basicAuth: &promauth.BasicAuthConfig{
					Password: promauth.NewSecret("secret"),
					Username: "user1",
				},
				tlsConfig: &promauth.TLSConfig{},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ac, err := buildConfig(tc.sdc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(ac, tc.expectedConfig) {
				t.Fatalf("unexpected result, got: %v, want: %v", ac, tc.expectedConfig)
			}
		})
	}

}
