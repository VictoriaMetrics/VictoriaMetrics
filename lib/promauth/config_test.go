package promauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewConfig(t *testing.T) {
	type args struct {
		baseDir         string
		az              *Authorization
		basicAuth       *BasicAuthConfig
		bearerToken     string
		bearerTokenFile string
		oauth           *OAuth2Config
		tlsConfig       *TLSConfig
	}
	tests := []struct {
		name         string
		args         args
		wantErr      bool
		expectHeader string
	}{
		{
			name: "OAuth2 config",
			args: args{
				oauth: &OAuth2Config{
					ClientID:     "some-id",
					ClientSecret: NewSecret("some-secret"),
					TokenURL:     "http://localhost:8511",
				},
			},
			expectHeader: "Bearer some-token",
		},
		{
			name: "OAuth2 config with file",
			args: args{
				oauth: &OAuth2Config{
					ClientID:         "some-id",
					ClientSecretFile: "testdata/test_secretfile.txt",
					TokenURL:         "http://localhost:8511",
				},
			},
			expectHeader: "Bearer some-token",
		},
		{
			name: "OAuth2 want err",
			args: args{
				oauth: &OAuth2Config{
					ClientID:         "some-id",
					ClientSecret:     NewSecret("some-secret"),
					ClientSecretFile: "testdata/test_secretfile.txt",
					TokenURL:         "http://localhost:8511",
				},
			},
			wantErr: true,
		},
		{
			name: "basic Auth config",
			args: args{
				basicAuth: &BasicAuthConfig{
					Username: "user",
					Password: NewSecret("password"),
				},
			},
			expectHeader: "Basic dXNlcjpwYXNzd29yZA==",
		},
		{
			name: "basic Auth config with file",
			args: args{
				basicAuth: &BasicAuthConfig{
					Username:     "user",
					PasswordFile: "testdata/test_secretfile.txt",
				},
			},
			expectHeader: "Basic dXNlcjpzZWNyZXQtY29udGVudA==",
		},
		{
			name: "want Authorization",
			args: args{
				az: &Authorization{
					Type:        "Bearer",
					Credentials: NewSecret("Value"),
				},
			},
			expectHeader: "Bearer Value",
		},
		{
			name: "token file",
			args: args{
				bearerTokenFile: "testdata/test_secretfile.txt",
			},
			expectHeader: "Bearer secret-content",
		},
		{
			name: "token with tls",
			args: args{
				bearerToken: "some-token",
				tlsConfig: &TLSConfig{
					InsecureSkipVerify: true,
				},
			},
			expectHeader: "Bearer some-token",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.args.oauth != nil {
				r := http.NewServeMux()
				r.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.Write([]byte(`{"access_token":"some-token","token_type": "Bearer"}`))

				})
				mock := httptest.NewServer(r)
				tt.args.oauth.TokenURL = mock.URL
			}
			got, err := NewConfig(tt.args.baseDir, tt.args.az, tt.args.basicAuth, tt.args.bearerToken, tt.args.bearerTokenFile, tt.args.oauth, tt.args.tlsConfig)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil {
				ah := got.GetAuthHeader()
				if ah != tt.expectHeader {
					t.Fatalf("unexpected auth header; got %q; want %q", ah, tt.expectHeader)
				}
			}

		})
	}
}
