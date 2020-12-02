package consul

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/fasthttp"
)

// apiConfig contains config for API server
// with consulWatcher service.
type apiConfig struct {
	tagSeparator  string
	consulWatcher *consulWatcher
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

	cw, err := newConsulWatcher(client, sdc, dc)
	if err != nil {
		return nil, fmt.Errorf("cannot start consul watcher: %w", err)
	}
	cfg := &apiConfig{
		tagSeparator:  tagSeparator,
		consulWatcher: cw,
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

// returns ServiceNodesState and version index.
func getServiceState(client *discoveryutils.Client, svc, baseArgs string, index uint64) ([]ServiceNode, uint64, error) {
	path := fmt.Sprintf("/v1/health/service/%s%s", svc, baseArgs)

	data, newIndex, err := getBlockingAPIResponse(client, path, index)
	if err != nil {
		return nil, index, err
	}
	sns, err := parseServiceNodes(data)
	if err != nil {
		return nil, index, err
	}
	return sns, newIndex, nil
}

// returns consul api response with new index version of object.
// https://www.consul.io/api-docs/features/blocking
func getBlockingAPIResponse(client *discoveryutils.Client, path string, index uint64) ([]byte, uint64, error) {
	path += "&index=" + strconv.FormatUint(index, 10)
	path = path + fmt.Sprintf("&wait=%s", watchTime)
	getMeta := func(resp *fasthttp.Response) {
		if ind := resp.Header.Peek("X-Consul-Index"); len(ind) > 0 {
			newIndex, err := strconv.ParseUint(string(ind), 10, 64)
			if err != nil {
				logger.Errorf("failed to parse consul index: %v", err)
				return
			}
			// reset index
			// https://www.consul.io/api-docs/features/blocking#implementation-details
			if index > newIndex {
				index = 0
				return
			}
			index = newIndex
		}
	}
	data, err := client.GetBlockingAPIResponse(path, getMeta)
	if err != nil {
		return nil, index, fmt.Errorf("failed query consul api path=%q, err=%w", path, err)
	}
	return data, index, nil
}
