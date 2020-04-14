package kubernetes

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/valyala/fasthttp"
)

func getAPIResponse(cfg *APIConfig, role, path string) ([]byte, error) {
	hcv, err := getHostClient(cfg.Server, cfg.AuthConfig)
	if err != nil {
		return nil, err
	}
	query := joinSelectors(role, cfg.Namespaces, cfg.Selectors)
	if len(query) > 0 {
		path += "?" + query
	}
	requestURL := hcv.apiServer + path
	var u fasthttp.URI
	u.Update(requestURL)
	var req fasthttp.Request
	req.SetRequestURIBytes(u.RequestURI())
	req.SetHost(hcv.hostPort)
	req.Header.Set("Accept-Encoding", "gzip")
	if hcv.ac != nil && hcv.ac.Authorization != "" {
		req.Header.Set("Authorization", hcv.ac.Authorization)
	}
	var resp fasthttp.Response
	// There is no need in calling DoTimeout, since the timeout is already set in hc.ReadTimeout above.
	if err := hcv.hc.Do(&req, &resp); err != nil {
		return nil, fmt.Errorf("cannot fetch %q: %s", requestURL, err)
	}
	var data []byte
	if ce := resp.Header.Peek("Content-Encoding"); string(ce) == "gzip" {
		dst, err := fasthttp.AppendGunzipBytes(nil, resp.Body())
		if err != nil {
			return nil, fmt.Errorf("cannot ungzip response from %q: %s", requestURL, err)
		}
		data = dst
	} else {
		data = append(data[:0], resp.Body()...)
	}
	statusCode := resp.StatusCode()
	if statusCode != fasthttp.StatusOK {
		return nil, fmt.Errorf("unexpected status code returned from %q: %d; expecting %d; response body: %q",
			requestURL, statusCode, fasthttp.StatusOK, data)
	}
	return data, nil
}

func getHostClient(apiServer string, ac *promauth.Config) (*hcValue, error) {
	k := hcKey{
		apiServer: apiServer,
		ac:        ac,
	}
	hcMapLock.Lock()
	defer hcMapLock.Unlock()

	if !hasHCMapCleaner {
		go hcMapCleaner()
		hasHCMapCleaner = true
	}
	hcv := hcMap[k]
	if hcv == nil {
		hcvNew, err := newHostClient(apiServer, ac)
		if err != nil {
			return hcv, fmt.Errorf("cannot create new HTTP client for %q: %s", apiServer, err)
		}
		hcMap[k] = hcvNew
		hcv = hcvNew
	}
	hcv.lastAccessTime = time.Now()
	return hcv, nil
}

func hcMapCleaner() {
	tc := time.NewTicker(15 * time.Minute)
	for currentTime := range tc.C {
		hcMapLock.Lock()
		for k, v := range hcMap {
			if currentTime.Sub(v.lastAccessTime) > 10*time.Minute {
				delete(hcMap, k)
			}
		}
		hcMapLock.Unlock()
	}
}

type hcKey struct {
	apiServer string
	ac        *promauth.Config
}

type hcValue struct {
	hc             *fasthttp.HostClient
	ac             *promauth.Config
	apiServer      string
	hostPort       string
	lastAccessTime time.Time
}

var (
	hasHCMapCleaner bool
	hcMap           = make(map[hcKey]*hcValue)
	hcMapLock       sync.Mutex
)

func newHostClient(apiServer string, ac *promauth.Config) (*hcValue, error) {
	if len(apiServer) == 0 {
		// Assume we run at k8s pod.
		// Discover apiServer and auth config according to k8s docs.
		// See https://kubernetes.io/docs/reference/access-authn-authz/service-accounts-admin/#service-account-admission-controller
		host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
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
		acNew, err := promauth.NewConfig("/", nil, "", "/var/run/secrets/kubernetes.io/serviceaccount/token", &tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("cannot initialize service account auth: %s; probably, `kubernetes_sd_config->api_server` is missing in Prometheus configs?", err)
		}
		ac = acNew
	}

	var u fasthttp.URI
	u.Update(apiServer)
	hostPort := string(u.Host())
	isTLS := string(u.Scheme()) == "https"
	var tlsCfg *tls.Config
	if isTLS && ac != nil {
		tlsCfg = ac.NewTLSConfig()
	}
	if !strings.Contains(hostPort, ":") {
		port := "80"
		if isTLS {
			port = "443"
		}
		hostPort = net.JoinHostPort(hostPort, port)
	}
	hc := &fasthttp.HostClient{
		Addr:                hostPort,
		Name:                "vm_promscrape/discovery",
		DialDualStack:       netutil.TCP6Enabled(),
		IsTLS:               isTLS,
		TLSConfig:           tlsCfg,
		ReadTimeout:         time.Minute,
		WriteTimeout:        10 * time.Second,
		MaxResponseBodySize: 300 * 1024 * 1024,
	}
	return &hcValue{
		hc:        hc,
		ac:        ac,
		apiServer: apiServer,
		hostPort:  hostPort,
	}, nil
}
