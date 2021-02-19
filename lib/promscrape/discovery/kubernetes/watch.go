package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

type watchResponse struct {
	Action string          `json:"type"`
	Object json.RawMessage `json:"object"`
}

func startWatcherByRole(role string, cfg *apiConfig, sdc *SDConfig) error {
	var err error
	switch role {
	case "pod":
		err = startWatchForObject(cfg, sdc, "pods", podsHandle)
	case "node":
		err = startWatchForObject(cfg, sdc, "nodes", nodesHandle)
	case "endpoints":
		err = startWatchForObject(cfg, sdc, "endpoints", func(config *apiConfig, response *watchResponse) {
			updateEndpointsCache(&config.watchCache, Endpoints{}, response)
		})
		if err != nil {
			return err
		}
		err = startWatchForObject(cfg, sdc, "pods", func(config *apiConfig, response *watchResponse) {
			updatePodCache(nil, &podsCache, Pod{}, response)
		})
		if err != nil {
			return err
		}
		err = startWatchForObject(cfg, sdc, "services", func(config *apiConfig, response *watchResponse) {
			updateServiceCache(nil, &servicesCache, Service{}, response)
		})
	case "service":
		err = startWatchForObject(cfg, sdc, "services", func(config *apiConfig, response *watchResponse) {
			updateServiceCache(&config.watchCache, &servicesCache, Service{}, response)
		})
	case "ingress":
		err = startWatchForObject(cfg, sdc, "ingresses", func(config *apiConfig, response *watchResponse) {
			updateIngressCache(&config.watchCache, Ingress{}, response)
		})
	case "endpointslices":
		err = startWatchForObject(cfg, sdc, "endpointslices", func(config *apiConfig, response *watchResponse) {
			updateEndpointSlicesCache(&config.watchCache, EndpointSlice{}, response)
		})
		if err != nil {
			return err
		}

		err = startWatchForObject(cfg, sdc, "pods", func(config *apiConfig, response *watchResponse) {
			updatePodCache(nil, &podsCache, Pod{}, response)
		})
		if err != nil {
			return err
		}

		err = startWatchForObject(cfg, sdc, "services", func(config *apiConfig, response *watchResponse) {
			updateServiceCache(nil, &servicesCache, Service{}, response)
		})
	default:
		err = fmt.Errorf("bug, unexpected role: %s", role)
	}
	return err
}

func stopWatcher(cfg *apiConfig) {
	t := time.NewTicker(time.Second * 2)
	for {
		select {
		case <-t.C:
			cfg.mu.Lock()
			lt := cfg.servicesLastAccessTime
			cfg.mu.Unlock()
			if time.Since(lt) > *SDCheckInterval*3 {
				startTime := time.Now()
				logger.Infof("stopping kubernetes watcher")
				cfg.wc.stop()
				logger.Infof("kubernetes watcher was stopped after: in %.3f seconds", time.Since(startTime).Seconds())
				return
			}
		}
	}
}

func startWatchForObject(cfg *apiConfig, sdc *SDConfig, objectName string, handleFunc func(config *apiConfig, response *watchResponse)) error {
	errCh := make(chan error, len(sdc.Namespaces.Names)*3+1)
	if len(sdc.Namespaces.Names) > 0 {
		for _, ns := range cfg.namespaces {
			// "/apis/discovery.k8s.io/v1beta1/
			path := fmt.Sprintf("/api/v1/watch/namespaces/%s/%s", ns, objectName)
			if objectName == "endpointslices" {
				path = fmt.Sprintf("/apis/discovery.k8s.io/v1beta1/watch/namespaces/%s/%s", ns, objectName)
			}

			query := joinSelectors(sdc.Role, nil, sdc.Selectors)
			if len(query) > 0 {
				path += "?" + query
			}
			go func(path string) {
				err := cfg.wc.startWatchForResource(path, func(object *watchResponse) {
					handleFunc(cfg, object)
				})
				if err != nil {
					errCh <- err
				}
			}(path)
		}
	} else {
		path := "/api/v1/watch/" + objectName
		if objectName == "endpointslices" {
			path = fmt.Sprintf("/apis/discovery.k8s.io/v1beta1/watch/%s", objectName)
		}
		query := joinSelectors(sdc.Role, sdc.Namespaces.Names, sdc.Selectors)
		if len(query) > 0 {
			path += "?" + query
		}
		go func() {
			err := cfg.wc.startWatchForResource(path, func(object *watchResponse) {
				handleFunc(cfg, object)
			})
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					logger.Errorf("got unexpected error: %v", err)
				}
				if len(errCh) < cap(errCh) {
					errCh <- err
				}
			}
		}()
	}
	select {
	case err := <-errCh:
		return err
	case <-time.After(time.Millisecond * 200):
	}
	return nil
}

type watchClient struct {
	c         *http.Client
	ac        *promauth.Config
	apiServer string
	hostPort  string
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

func (wc *watchClient) stop() {
	if wc.cancel != nil {
		wc.cancel()
	}
	wc.wg.Wait()
}

func (wc *watchClient) startWatchForResource(path string, watchFunc func(object *watchResponse)) error {
	ctx, cancel := context.WithCancel(context.Background())
	wc.cancel = cancel
	for {
		wc.wg.Add(1)
		err := wc.getStreamAPIResponse(ctx, path, watchFunc)
		if err != io.EOF {
			return err
		}
	}
}
func (wc *watchClient) getStreamAPIResponse(ctx context.Context, path string, watchFunc func(object *watchResponse)) error {
	defer wc.wg.Done()
	req, err := http.NewRequestWithContext(ctx, "GET", wc.apiServer+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept-Encoding", "gzip")
	if wc.ac != nil && wc.ac.Authorization != "" {
		req.Header.Set("Authorization", wc.ac.Authorization)
	}
	resp, err := wc.c.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	r := NewJSONFramedReader(resp.Body)
	for {
		b := make([]byte, 1024, 1024)
		b, err := readJSONObject(r, b)
		if err != nil {
			logger.Errorf("got unexpected reading response frame error: %v", err)
			return err
		}
		var rObject watchResponse
		err = json.Unmarshal(b, &rObject)
		if err != nil {
			logger.Errorf("failed to parse watch api response as json, err %v, response: %v", err, string(b))
			continue
		}
		watchFunc(&rObject)
	}
}

func readJSONObject(r io.Reader, b []byte) ([]byte, error) {
	offset := 0
	for {
		n, err := r.Read(b[offset:])
		if err == io.ErrShortBuffer {
			if n == 0 {
				return nil, fmt.Errorf("got short buffer with n=0, cap=%d", cap(b))
			}
			// double buffer..
			b = bytesutil.Resize(b, len(b)*2)
			offset += n
			continue
		}
		if err != nil {
			return nil, err
		}
		offset += n
		break
	}
	return b[:offset], nil
}

func newWatchClient(sdc *SDConfig, baseDir string) (*watchClient, error) {
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
	var proxy func(*http.Request) (*url.URL, error)
	if proxyURL := sdc.ProxyURL.URL(); proxyURL != nil {
		proxy = http.ProxyURL(proxyURL)
	}
	c := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:     ac.NewTLSConfig(),
			Proxy:               proxy,
			TLSHandshakeTimeout: 10 * time.Second,
			IdleConnTimeout:     2 * time.Minute,
		},
	}
	wc := watchClient{
		c:         c,
		apiServer: apiServer,
		ac:        ac,
	}
	return &wc, nil
}
