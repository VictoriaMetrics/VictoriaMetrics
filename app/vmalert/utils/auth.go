package utils

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// AuthConfig returns promauth.Config based on the given params
func AuthConfig(baUser, baPass, baFile, bearerToken, bearerTokenFile string) (*promauth.Config, error) {
	var baCfg *promauth.BasicAuthConfig
	if baUser != "" || baPass != "" || baFile != "" {
		baCfg = &promauth.BasicAuthConfig{
			Username:     baUser,
			Password:     promauth.NewSecret(baPass),
			PasswordFile: baFile,
		}
	}
	return promauth.NewConfig(".", nil, baCfg, bearerToken, bearerTokenFile, nil, nil)
}
