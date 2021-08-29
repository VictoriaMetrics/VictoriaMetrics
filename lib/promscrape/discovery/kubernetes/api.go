package kubernetes

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// apiConfig contains config for API server
type apiConfig struct {
	aw *apiWatcher
}

func newAPIConfig(sdc *SDConfig, baseDir string, swcFunc ScrapeWorkConstructorFunc) (*apiConfig, error) {
	role := sdc.role()
	switch role {
	case "node", "pod", "service", "endpoints", "endpointslice", "ingress":
	default:
		return nil, fmt.Errorf("unexpected `role`: %q; must be one of `node`, `pod`, `service`, `endpoints`, `endpointslice` or `ingress`", role)
	}
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
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
		acNew, err := promauth.NewConfig(".", nil, nil, "", "/var/run/secrets/kubernetes.io/serviceaccount/token", nil, &tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize service account auth: %w; probably, `kubernetes_sd_config->api_server` is missing in Prometheus configs?", err)
		}
		ac = acNew
	}
	if !strings.Contains(apiServer, "://") {
		proto := "http"
		if sdc.HTTPClientConfig.TLSConfig != nil {
			proto = "https"
		}
		apiServer = proto + "://" + apiServer
	}
	for strings.HasSuffix(apiServer, "/") {
		apiServer = apiServer[:len(apiServer)-1]
	}
	aw := newAPIWatcher(apiServer, ac, sdc, swcFunc)
	cfg := &apiConfig{
		aw: aw,
	}
	return cfg, nil
}
