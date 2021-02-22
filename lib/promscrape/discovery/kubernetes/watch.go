package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
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

func startWatcherByRole(ctx context.Context, role string, cfg *apiConfig) []map[string]string {
	var ms []map[string]string
	switch role {
	case "pod":
		startWatchForObject(ctx, cfg, "pods", func(wr *watchResponse) {
			var p Pod
			if err := json.Unmarshal(wr.Object, &p); err != nil {
				return
			}
			processPods(cfg, &p, wr.Action)
		}, func(bytes []byte) (string, error) {
			pods, err := parsePodList(bytes)
			if err != nil {
				return "", err
			}
			for _, pod := range pods.Items {
				ms = pod.appendTargetLabels(ms)
				processPods(cfg, &pod, "ADDED")
			}
			return pods.Metadata.ResourceVersion, nil
		})
	case "node":
		startWatchForObject(ctx, cfg, "nodes", func(wr *watchResponse) {
			var n Node
			if err := json.Unmarshal(wr.Object, &n); err != nil {
				return
			}
			processNode(cfg, &n, wr.Action)
		}, func(bytes []byte) (string, error) {
			nodes, err := parseNodeList(bytes)
			if err != nil {
				return "", err
			}
			for _, node := range nodes.Items {
				processNode(cfg, &node, "ADDED")
				ms = node.appendTargetLabels(ms)
			}
			return nodes.Metadata.ResourceVersion, nil
		})
	case "endpoints":
		startWatchForObject(ctx, cfg, "pods", func(wr *watchResponse) {
			var p Pod
			if err := json.Unmarshal(wr.Object, &p); err != nil {
				return
			}
			updatePodCache(&podsCache, &p, wr.Action)
			if wr.Action == "MODIFIED" {
				eps, ok := endpointsCache.Load(p.key())
				if ok {
					ep := eps.(*Endpoints)
					processEndpoints(cfg, ep, wr.Action)
				}
			}
		}, func(bytes []byte) (string, error) {
			pods, err := parsePodList(bytes)
			if err != nil {
				return "", err
			}
			for _, pod := range pods.Items {
				updatePodCache(&podsCache, &pod, "ADDED")
			}
			return pods.Metadata.ResourceVersion, nil
		})
		startWatchForObject(ctx, cfg, "services", func(wr *watchResponse) {
			var svc Service
			if err := json.Unmarshal(wr.Object, &svc); err != nil {
				return
			}
			updateServiceCache(&servicesCache, &svc, wr.Action)
			if wr.Action == "MODIFIED" {
				linkedEps, ok := endpointsCache.Load(svc.key())
				if ok {
					ep := linkedEps.(*Endpoints)
					processEndpoints(cfg, ep, wr.Action)
				}
			}
		}, func(bytes []byte) (string, error) {
			svcs, err := parseServiceList(bytes)
			if err != nil {
				return "", err
			}
			for _, svc := range svcs.Items {
				updateServiceCache(&servicesCache, &svc, "ADDED")
			}
			return svcs.Metadata.ResourceVersion, nil
		})
		startWatchForObject(ctx, cfg, "endpoints", func(wr *watchResponse) {
			var eps Endpoints
			if err := json.Unmarshal(wr.Object, &eps); err != nil {
				return
			}
			processEndpoints(cfg, &eps, wr.Action)
			updateEndpointsCache(&endpointsCache, &eps, wr.Action)
		}, func(bytes []byte) (string, error) {
			eps, err := parseEndpointsList(bytes)
			if err != nil {
				return "", err
			}
			for _, ep := range eps.Items {
				ms = ep.appendTargetLabels(ms, &podsCache, &servicesCache)
				processEndpoints(cfg, &ep, "ADDED")
				updateEndpointsCache(&endpointsCache, &ep, "ADDED")
			}
			return eps.Metadata.ResourceVersion, nil
		})
	case "service":
		startWatchForObject(ctx, cfg, "services", func(wr *watchResponse) {
			var svc Service
			if err := json.Unmarshal(wr.Object, &svc); err != nil {
				return
			}
			processService(cfg, &svc, wr.Action)
		}, func(bytes []byte) (string, error) {
			svcs, err := parseServiceList(bytes)
			if err != nil {
				return "", err
			}
			for _, svc := range svcs.Items {
				processService(cfg, &svc, "ADDED")
				ms = svc.appendTargetLabels(ms)
			}
			return svcs.Metadata.ResourceVersion, nil
		})
	case "ingress":
		startWatchForObject(ctx, cfg, "ingresses", func(wr *watchResponse) {
			var ig Ingress
			if err := json.Unmarshal(wr.Object, &ig); err != nil {
				return
			}
			processIngress(cfg, &ig, wr.Action)
		}, func(bytes []byte) (string, error) {
			igs, err := parseIngressList(bytes)
			if err != nil {
				return "", err
			}
			for _, ig := range igs.Items {
				processIngress(cfg, &ig, "ADDED")
				ms = ig.appendTargetLabels(ms)
			}
			return igs.Metadata.ResourceVersion, nil
		})
	case "endpointslices":
		startWatchForObject(ctx, cfg, "pods", func(wr *watchResponse) {
			var p Pod
			if err := json.Unmarshal(wr.Object, &p); err != nil {
				return
			}
			updatePodCache(&podsCache, &p, wr.Action)
			if wr.Action == "MODIFIED" {
				eps, ok := endpointSlicesCache.Load(p.key())
				if ok {
					ep := eps.(*EndpointSlice)
					processEndpointSlices(cfg, ep, wr.Action)
				}
			}
		}, func(bytes []byte) (string, error) {
			pods, err := parsePodList(bytes)
			if err != nil {
				return "", err
			}
			for _, pod := range pods.Items {
				updatePodCache(&podsCache, &pod, "ADDED")
			}
			return pods.Metadata.ResourceVersion, nil
		})
		startWatchForObject(ctx, cfg, "services", func(wr *watchResponse) {
			var svc Service
			if err := json.Unmarshal(wr.Object, &svc); err != nil {
				return
			}
			updateServiceCache(&servicesCache, &svc, wr.Action)
			if wr.Action == "MODIFIED" {
				linkedEps, ok := endpointSlicesCache.Load(svc.key())
				if ok {
					ep := linkedEps.(*EndpointSlice)
					processEndpointSlices(cfg, ep, wr.Action)
				}
			}
		}, func(bytes []byte) (string, error) {
			svcs, err := parseServiceList(bytes)
			if err != nil {
				return "", err
			}
			for _, svc := range svcs.Items {
				updateServiceCache(&servicesCache, &svc, "ADDED")
			}
			return svcs.Metadata.ResourceVersion, nil
		})
		startWatchForObject(ctx, cfg, "endpointslices", func(wr *watchResponse) {
			var eps EndpointSlice
			if err := json.Unmarshal(wr.Object, &eps); err != nil {
				return
			}
			processEndpointSlices(cfg, &eps, wr.Action)
			updateEndpointsSliceCache(&endpointSlicesCache, &eps, wr.Action)
		}, func(bytes []byte) (string, error) {
			epss, err := parseEndpointSlicesList(bytes)
			if err != nil {
				return "", err
			}
			for _, eps := range epss.Items {
				ms = eps.appendTargetLabels(ms, &podsCache, &servicesCache)
				processEndpointSlices(cfg, &eps, "ADDED")
			}
			return epss.Metadata.ResourceVersion, nil
		})
	default:
		logger.Errorf("unexpected role: %s", role)
	}
	return ms
}

func startWatchForObject(ctx context.Context, cfg *apiConfig, objectName string, wh func(wr *watchResponse), getSync func([]byte) (string, error)) {
	if len(cfg.namespaces) > 0 {
		for _, ns := range cfg.namespaces {
			// "/apis/discovery.k8s.io/v1beta1/
			path := fmt.Sprintf("/api/v1/namespaces/%s/%s", ns, objectName)
			if objectName == "endpointslices" {
				path = fmt.Sprintf("/apis/discovery.k8s.io/v1beta1/namespaces/%s/%s", ns, objectName)
			}
			query := joinSelectors(objectName, nil, cfg.selectors)
			if len(query) > 0 {
				path += "?" + query
			}
			data, err := cfg.wc.getBlockingAPIResponse(path)
			if err != nil {
				logger.Errorf("cannot get latest resource version: %v", err)
			}
			version, err := getSync(data)
			if err != nil {
				logger.Errorf("cannot get latest resource version: %v", err)
			}
			cfg.wc.wg.Add(1)
			go func(path string) {
				cfg.wc.startWatchForResource(ctx, path, wh, version)
			}(path)
		}
	} else {
		path := "/api/v1/" + objectName
		if objectName == "endpointslices" {
			path = fmt.Sprintf("/apis/discovery.k8s.io/v1beta1/%s", objectName)
		}
		query := joinSelectors(objectName, nil, cfg.selectors)
		if len(query) > 0 {
			path += "?" + query
		}
		data, err := cfg.wc.getBlockingAPIResponse(path)
		if err != nil {
			logger.Errorf("cannot get latest resource version: %v", err)
		}
		version, err := getSync(data)
		if err != nil {
			logger.Errorf("cannot get latest resource version: %v", err)
		}
		cfg.wc.wg.Add(1)
		go func() {
			cfg.wc.startWatchForResource(ctx, path, wh, version)
		}()
	}
	return
}

type watchClient struct {
	c         *http.Client
	ac        *promauth.Config
	apiServer string
	wg        *sync.WaitGroup
}

func (wc *watchClient) startWatchForResource(ctx context.Context, path string, wh func(wr *watchResponse), initResourceVersion string) {
	defer wc.wg.Done()
	path += "?watch=1"
	maxBackOff := time.Minute
	backoff := time.Second
	for {
		err := wc.getStreamAPIResponse(ctx, path, initResourceVersion, wh)
		if errors.Is(err, context.Canceled) {
			logger.Infof("context canceled")
			return
		}
		if !errors.Is(err, io.EOF) {
			logger.Errorf("got unexpected error: %v", err)
		}
		initResourceVersion = ""
		if backoff < maxBackOff {
			backoff += time.Second * 3
		}
		time.Sleep(backoff)
	}
}

func (wc *watchClient) getBlockingAPIResponse(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", wc.apiServer+path, nil)
	if err != nil {
		return nil, err
	}
	//req.Header.Set("Accept-Encoding", "gzip")
	if wc.ac != nil && wc.ac.Authorization != "" {
		req.Header.Set("Authorization", wc.ac.Authorization)
	}
	resp, err := wc.c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get unexpected code: %d, at blocking api request path: %q", resp.StatusCode, path)
	}
	return ioutil.ReadAll(resp.Body)
}

func (wc *watchClient) getStreamAPIResponse(ctx context.Context, path, resouceVersion string, wh func(wr *watchResponse)) error {
	if resouceVersion != "" {
		path += "&resourceVersion=" + resouceVersion
	}
	req, err := http.NewRequestWithContext(ctx, "GET", wc.apiServer+path, nil)
	if err != nil {
		return err
	}
	//req.Header.Set("Accept-Encoding", "gzip")
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
		b := make([]byte, 1024)
		b, err := readJSONObject(r, b)
		if err != nil {
			return err
		}
		var rObject watchResponse
		err = json.Unmarshal(b, &rObject)
		if err != nil {
			logger.Errorf("failed to parse watch api response as json, err %v, response: %v", err, string(b))
			continue
		}
		wh(&rObject)
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

func newWatchClient(wg *sync.WaitGroup, sdc *SDConfig, baseDir string) (*watchClient, error) {
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
		wg:        wg,
	}
	return &wc, nil
}
