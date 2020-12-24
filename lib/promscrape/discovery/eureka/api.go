package eureka

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

var configMap = discoveryutils.NewConfigMap()

type apiConfig struct {
	client *discoveryutils.Client
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	token := ""
	if sdc.Token != nil {
		token = *sdc.Token
	}
	var ba *promauth.BasicAuthConfig
	if len(sdc.Username) > 0 {
		ba = &promauth.BasicAuthConfig{
			Username: sdc.Username,
			Password: sdc.Password,
		}
		token = ""
	}
	ac, err := promauth.NewConfig(baseDir, ba, token, "", sdc.TLSConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	apiServer := sdc.Server
	if apiServer == "" {
		apiServer = "localhost:8080/eureka/v2"
	}
	if !strings.Contains(apiServer, "://") {
		scheme := sdc.Scheme
		if scheme == "" {
			scheme = "http"
		}
		apiServer = scheme + "://" + apiServer
	}
	client, err := discoveryutils.NewClient(apiServer, ac, sdc.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}
	cfg := &apiConfig{
		client: client,
	}
	return cfg, nil
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil

}

func getAPIResponse(cfg *apiConfig, path string) ([]byte, error) {
	return cfg.client.GetAPIResponse(path)
}

func parseAPIResponse(data []byte) (*applications, error) {
	var apps applications
	if err := xml.Unmarshal(data, &apps); err != nil {
		return nil, fmt.Errorf("failed parse eureka api response: %q, err: %w", data, err)
	}
	return &apps, nil
}
