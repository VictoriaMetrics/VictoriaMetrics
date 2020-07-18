package consul

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// apiConfig contains config for API server
type apiConfig struct {
	client       *discoveryutils.Client
	tagSeparator string
	services     []string
	tags         []string
	datacenter   string
	allowStale   bool
	nodeMeta     map[string]string
}

var configMap = discoveryutils.NewConfigMap()

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	token, err := getToken(sdc.Token)
	if err != nil {
		return nil, err
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
		apiServer = "localhost:8500"
	}
	if !strings.Contains(apiServer, "://") {
		scheme := sdc.Scheme
		if scheme == "" {
			scheme = "http"
		}
		apiServer = scheme + "://" + apiServer
	}
	client, err := discoveryutils.NewClient(apiServer, ac)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}
	tagSeparator := ","
	if sdc.TagSeparator != nil {
		tagSeparator = *sdc.TagSeparator
	}
	dc, err := getDatacenter(client, sdc.Datacenter)
	if err != nil {
		return nil, err
	}
	cfg := &apiConfig{
		client: client,

		tagSeparator: tagSeparator,
		services:     sdc.Services,
		tags:         sdc.Tags,
		datacenter:   dc,
		allowStale:   sdc.AllowStale,
		nodeMeta:     sdc.NodeMeta,
	}
	return cfg, nil
}

func getToken(token *string) (string, error) {
	if token != nil {
		return *token, nil
	}
	if tokenFile := os.Getenv("CONSUL_HTTP_TOKEN_FILE"); tokenFile != "" {
		data, err := ioutil.ReadFile(tokenFile)
		if err != nil {
			return "", fmt.Errorf("cannot read consul token file %q; probably, `token` arg is missing in `consul_sd_config`? error: %w", tokenFile, err)
		}
		return string(data), nil
	}
	t := os.Getenv("CONSUL_HTTP_TOKEN")
	// Allow empty token - it shouls work if authorization is disabled in Consul
	return t, nil
}

func getDatacenter(client *discoveryutils.Client, dc string) (string, error) {
	if dc != "" {
		return dc, nil
	}
	// See https://www.consul.io/api/agent.html#read-configuration
	data, err := client.GetAPIResponse("/v1/agent/self")
	if err != nil {
		return "", fmt.Errorf("cannot query consul agent info: %w", err)
	}
	a, err := parseAgent(data)
	if err != nil {
		return "", err
	}
	return a.Config.Datacenter, nil
}

func getAPIResponse(cfg *apiConfig, path string) ([]byte, error) {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	path += fmt.Sprintf("%sdc=%s", separator, url.QueryEscape(cfg.datacenter))
	if cfg.allowStale {
		// See https://www.consul.io/api/features/consistency
		path += "&stale"
	}
	if len(cfg.nodeMeta) > 0 {
		for k, v := range cfg.nodeMeta {
			path += fmt.Sprintf("&node-meta=%s", url.QueryEscape(k+":"+v))
		}
	}
	return cfg.client.GetAPIResponse(path)
}
