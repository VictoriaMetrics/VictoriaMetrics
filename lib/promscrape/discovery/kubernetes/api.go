package kubernetes

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// apiConfig contains config for API server
type apiConfig struct {
	client     *discoveryutils.Client
	namespaces []string
	selectors  []Selector
	wc         *watchClient
	podCache   sync.Map
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
	ac, err := promauth.NewConfig(baseDir, sdc.BasicAuth, sdc.BearerToken, sdc.BearerTokenFile, sdc.TLSConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	apiServer := sdc.APIServer
	if len(apiServer) == 0 {
		// Assume we run at k8s pod.
		// Discover apiServer and auth config according to k8s docs.
		// See https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#service-account-admission-controller
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		port := os.Getenv("KUBERNETES_SERVICE_PORT")
		if len(host) == 0 {
			return nil, fmt.Errorf("cannot find KUBERNETES_SERVICE_HOST env var; it must be defined when running in k8s; " +
				"probably, `kubernetes_sd_config->api_server` is missing in Prometheus configs?")
		}
		if len(port) == 0 {
			return nil, fmt.Errorf("cannot find KUBERNETES_SERVICE_PORT env var; it must be defined when running in k8s; "+
				"KUBERNETES_SERVICE_HOST=%q", host)
		}
		apiServer = "https://" + net.JoinHostPort(host, port)
		tlsConfig := promauth.TLSConfig{
			CAFile: "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
		}
		acNew, err := promauth.NewConfig(".", nil, "", "/var/run/secrets/kubernetes.io/serviceaccount/token", &tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize service account auth: %w; probably, `kubernetes_sd_config->api_server` is missing in Prometheus configs?", err)
		}
		ac = acNew
	}
	client, err := discoveryutils.NewClient(apiServer, ac, sdc.ProxyURL)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}
	wc, err := newWatchClient(sdc, "")
	if err != nil {
		return nil, err
	}
	cfg := &apiConfig{
		client:     client,
		wc:         wc,
		namespaces: sdc.Namespaces.Names,
		selectors:  sdc.Selectors,
	}
	switch sdc.Role {
	case "pod":
		if len(sdc.Namespaces.Names) > 0 {
			for _, ns := range cfg.namespaces {
				path := fmt.Sprintf("/api/v1/watch/namespaces/%s/pods", ns)
				query := joinSelectors(sdc.Role, nil, sdc.Selectors)
				if len(query) > 0 {
					path += "?" + query
				}
				logger.Infof("path: %v", path)
				err = startWatchForPods(cfg.wc, path, func(resp *watchResponse) {
					podsHandle(cfg, resp)
				})
				if err != nil {
					return nil, err
				}
			}
			break
		}
		path := "/api/v1/watch/pods"
		query := joinSelectors(sdc.Role, sdc.Namespaces.Names, sdc.Selectors)
		if len(query) > 0 {
			path += "?" + query
		}
		err = startWatchForPods(cfg.wc, path, func(resp *watchResponse) {
			podsHandle(cfg, resp)
		})
	default:
		err = fmt.Errorf("not implemented role: %s", sdc.Role)
	}
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func getAPIResponse(cfg *apiConfig, role, path string) ([]byte, error) {
	query := joinSelectors(role, cfg.namespaces, cfg.selectors)
	if len(query) > 0 {
		path += "?" + query
	}
	return cfg.client.GetAPIResponse(path)
}
