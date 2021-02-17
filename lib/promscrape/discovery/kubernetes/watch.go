package kubernetes

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type watchResponse struct {
	Action string          `json:"type"`
	Object json.RawMessage `json:"object"`
}

func startWatchForPods(client *watchClient, path string, handleFunc func(resp *watchResponse)) error {
	errCh := make(chan error, 1)
	go func() {

		err := client.startWatchForResource(path, handleFunc)
		if err != nil {
			logger.Errorf("got error: %v", err)
			if len(errCh) < cap(errCh) {
				errCh <- err
			}
		}
	}()
	select {
	case err := <-errCh:
		return err
	case <-time.After(time.Second * 2):
	}
	return nil
}

type watchClient struct {
	c         *http.Client
	ac        *promauth.Config
	apiServer string
	hostPort  string
}

func (wc *watchClient) startWatchForResource(path string, watchFunc func(object *watchResponse)) error {
	for {
		err := wc.getStreamAPIResponse(path, watchFunc)
		if err != io.EOF {
			return err
		}
	}
}
func (wc *watchClient) getStreamAPIResponse(path string, watchFunc func(object *watchResponse)) error {
	req, err := http.NewRequest("GET", wc.apiServer+path, nil)
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
		b, err := readJsonObject(r, b)
		if err != nil {
			logger.Errorf("got error: %v", err)
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

func readJsonObject(r io.Reader, b []byte) ([]byte, error) {
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
	sc := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:     ac.NewTLSConfig(),
			Proxy:               proxy,
			TLSHandshakeTimeout: 10 * time.Second,
			IdleConnTimeout:     2 * time.Minute,
		},
	}
	wc := watchClient{
		c:         sc,
		apiServer: apiServer,
		ac:        ac,
	}
	return &wc, nil
}
