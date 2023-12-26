package hetzner

import (
	"fmt"
	"os"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

var configMap = discoveryutils.NewConfigMap()

type apiConfig struct {
	client *discoveryutils.Client
	role   string
	port   int
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	hcc := sdc.HTTPClientConfig

	var apiServer string
	switch sdc.Role {
	case "robot":
		apiServer = "https://robot-ws.your-server.de"
		if hcc.BasicAuth == nil {
			return nil, fmt.Errorf("basic_auth must be set when role is `%q`", sdc.Role)
		}
	case "hcloud":
		apiServer = "https://api.hetzner.cloud/v1"
		token, err := GetToken(sdc.Token)
		if err != nil {
			return nil, err
		}
		if token != "" {
			if hcc.BearerToken != nil {
				return nil, fmt.Errorf("cannot set both token and bearer_token configs")
			}
			hcc.BearerToken = promauth.NewSecret(token)
		}
	default:
		return nil, fmt.Errorf("skipping unexpected role=%q; must be one of `robot` or `hcloud`", sdc.Role)
	}

	ac, err := hcc.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}
	client, err := discoveryutils.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}
	port := 80
	if sdc.Port != nil {
		port = *sdc.Port
	}
	cfg := &apiConfig{
		client: client,
		role:   sdc.Role,
		port:   port,
	}
	return cfg, nil
}

// GetToken returns Hcloud token.
func GetToken(token *promauth.Secret) (string, error) {
	if token != nil {
		return token.String(), nil
	}
	if tokenFile := os.Getenv("HCLOUD_TOKEN_FILE"); tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			return "", fmt.Errorf("cannot read hcloud token file %q; probably, `token` arg is missing in `hetzner_sd_config`? error: %w", tokenFile, err)
		}
		return string(data), nil
	}
	t := os.Getenv("HCLOUD_TOKEN")
	//to-do retrict empty token.
	return t, nil
}
