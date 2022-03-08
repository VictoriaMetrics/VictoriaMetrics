package utils

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

type AuthConfigOptions func(config *promauth.HTTPClientConfig)

// AuthConfig returns promauth.Config based on the given params
func AuthConfig(filterOptions ...AuthConfigOptions) (*promauth.Config, error) {
	authCfg := &promauth.HTTPClientConfig{}
	for _, option := range filterOptions {
		option(authCfg)
	}

	return authCfg.NewConfig(".")
}

func WithAuthorization(authType, credentials, credentialsFile string) AuthConfigOptions {
	return func(config *promauth.HTTPClientConfig) {
		authCfg := &promauth.Authorization{}
		if authType != "" {
			authCfg.Type = authType
		}
		if credentials != "" {
			authCfg.Credentials = promauth.NewSecret(credentials)
		}
		if credentialsFile != "" {
			authCfg.CredentialsFile = credentialsFile
		}
		if authCfg.Type != "" || authCfg.Credentials.String() != "" || authCfg.CredentialsFile != "" {
			config.Authorization = authCfg
		}
	}
}

func WithBasicAuth(username, password, passwordFile string) AuthConfigOptions {
	return func(config *promauth.HTTPClientConfig) {
		if username != "" || password != "" || passwordFile != "" {
			config.BasicAuth = &promauth.BasicAuthConfig{
				Username:     username,
				Password:     promauth.NewSecret(password),
				PasswordFile: passwordFile,
			}
		}
	}
}

func WithBearer(token, tokenFile string) AuthConfigOptions {
	return func(config *promauth.HTTPClientConfig) {
		if token != "" {
			config.BearerToken = promauth.NewSecret(token)
		}
		if tokenFile != "" {
			config.BearerTokenFile = tokenFile
		}
	}
}
