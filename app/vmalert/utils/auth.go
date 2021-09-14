package utils

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func AuthConfig(baUser, baPass, baFile, bearerToken, bearerTokenFile string) (*promauth.Config, error) {
	var baCfg *promauth.BasicAuthConfig
	if baUser != "" || baPass != "" || baFile != "" {
		baCfg = &promauth.BasicAuthConfig{
			Username:     baUser,
			Password:     baPass,
			PasswordFile: baFile,
		}
	}
	return promauth.NewConfig(".", nil, baCfg, bearerToken, bearerTokenFile, nil, nil)
}
