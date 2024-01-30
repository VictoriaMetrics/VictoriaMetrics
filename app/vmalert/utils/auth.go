package utils

import (
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// AuthConfigOptions options which helps build promauth.Config
type AuthConfigOptions func(config *promauth.HTTPClientConfig)

// AuthConfig returns promauth.Config based on the given params
func AuthConfig(filterOptions ...AuthConfigOptions) (*promauth.Config, error) {
	authCfg := &promauth.HTTPClientConfig{}
	for _, option := range filterOptions {
		option(authCfg)
	}

	return authCfg.NewConfig(".")
}

// WithBasicAuth returns AuthConfigOptions and initialized promauth.BasicAuthConfig based on given params
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

// WithBearer returns AuthConfigOptions and set BearerToken or BearerTokenFile based on given params
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

// WithOAuth returns AuthConfigOptions and set OAuth params based on given params
func WithOAuth(clientID, clientSecret, clientSecretFile, tokenURL, scopes string, endpointParams map[string]string) AuthConfigOptions {
	return func(config *promauth.HTTPClientConfig) {
		if clientSecretFile != "" || clientSecret != "" {
			config.OAuth2 = &promauth.OAuth2Config{
				ClientID:         clientID,
				ClientSecret:     promauth.NewSecret(clientSecret),
				ClientSecretFile: clientSecretFile,
				EndpointParams:   endpointParams,
				TokenURL:         tokenURL,
				Scopes:           strings.Split(scopes, ";"),
			}
		}
	}
}

// WithHeaders returns AuthConfigOptions and set Headers based on the given params
func WithHeaders(headers string) AuthConfigOptions {
	return func(config *promauth.HTTPClientConfig) {
		if headers != "" {
			config.Headers = strings.Split(headers, "^^")
		}
	}
}
