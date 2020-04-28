package gce

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"golang.org/x/oauth2/google"
)

type apiConfig struct {
	client       *http.Client
	zones        []string
	project      string
	filter       string
	tagSeparator string
	port         int
}

func getAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	apiConfigMapLock.Lock()
	defer apiConfigMapLock.Unlock()

	if !hasAPIConfigMapCleaner {
		hasAPIConfigMapCleaner = true
		go apiConfigMapCleaner()
	}

	e := apiConfigMap[sdc]
	if e != nil {
		e.lastAccessTime = time.Now()
		return e.cfg, nil
	}
	cfg, err := newAPIConfig(sdc)
	if err != nil {
		return nil, err
	}
	apiConfigMap[sdc] = &apiConfigMapEntry{
		cfg:            cfg,
		lastAccessTime: time.Now(),
	}
	return cfg, nil
}

func apiConfigMapCleaner() {
	tc := time.NewTicker(15 * time.Minute)
	for currentTime := range tc.C {
		apiConfigMapLock.Lock()
		for k, e := range apiConfigMap {
			if currentTime.Sub(e.lastAccessTime) > 10*time.Minute {
				delete(apiConfigMap, k)
			}
		}
		apiConfigMapLock.Unlock()
	}
}

type apiConfigMapEntry struct {
	cfg            *apiConfig
	lastAccessTime time.Time
}

var (
	apiConfigMap           = make(map[*SDConfig]*apiConfigMapEntry)
	apiConfigMapLock       sync.Mutex
	hasAPIConfigMapCleaner bool
)

func newAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	ctx := context.Background()
	client, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/compute.readonly")
	if err != nil {
		return nil, fmt.Errorf("cannot create oauth2 client for gce: %s", err)
	}
	project := sdc.Project
	if len(project) == 0 {
		proj, err := getCurrentProject()
		if err != nil {
			return nil, fmt.Errorf("cannot determine the current project; make sure `vmagent` runs inside GCE; error: %s", err)
		}
		project = proj
		logger.Infof("autodetected the current GCE project: %q", project)
	}
	zones := sdc.Zone.zones
	if len(zones) == 0 {
		// Autodetect the current zone.
		zone, err := getCurrentZone()
		if err != nil {
			return nil, fmt.Errorf("cannot determine the current zone; make sure `vmagent` runs inside GCE; error: %s", err)
		}
		zones = append(zones, zone)
		logger.Infof("autodetected the current GCE zone: %q", zone)
	} else if len(zones) == 1 && zones[0] == "*" {
		// Autodetect zones for project.
		zs, err := getZonesForProject(client, project, sdc.Filter)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain zones for project %q: %s", project, err)
		}
		zones = zs
		logger.Infof("autodetected all the zones for the GCE project %q: %q", project, zones)
	}
	tagSeparator := ","
	if sdc.TagSeparator != nil {
		tagSeparator = *sdc.TagSeparator
	}
	port := 80
	if sdc.Port != nil {
		port = *sdc.Port
	}
	return &apiConfig{
		client:       client,
		zones:        zones,
		project:      project,
		filter:       sdc.Filter,
		tagSeparator: tagSeparator,
		port:         port,
	}, nil
}

func getAPIResponse(client *http.Client, apiURL, filter, pageToken string) ([]byte, error) {
	apiURL = appendNonEmptyQueryArg(apiURL, "filter", filter)
	apiURL = appendNonEmptyQueryArg(apiURL, "pageToken", pageToken)
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("cannot query %q: %s", apiURL, err)
	}
	return readResponseBody(resp, apiURL)
}

func readResponseBody(resp *http.Response, apiURL string) ([]byte, error) {
	data, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot read response from %q: %s", apiURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code for %q; got %d; want %d; response body: %q",
			apiURL, resp.StatusCode, http.StatusOK, data)
	}
	return data, nil
}

func appendNonEmptyQueryArg(apiURL, argName, argValue string) string {
	if len(argValue) == 0 {
		return apiURL
	}
	prefix := "?"
	if strings.Contains(apiURL, "?") {
		prefix = "&"
	}
	return apiURL + fmt.Sprintf("%s%s=%s", prefix, url.QueryEscape(argName), url.QueryEscape(argValue))
}

func getCurrentZone() (string, error) {
	// See https://cloud.google.com/compute/docs/storing-retrieving-metadata#default
	data, err := getGCEMetadata("instance/zone")
	if err != nil {
		return "", err
	}
	parts := strings.Split(string(data), "/")
	if len(parts) != 4 {
		return "", fmt.Errorf("unexpected data returned from GCE; it must contain something like `projects/projectnum/zones/zone`; data: %q", data)
	}
	return parts[3], nil
}

func getCurrentProject() (string, error) {
	// See https://cloud.google.com/compute/docs/storing-retrieving-metadata#default
	data, err := getGCEMetadata("project/project-id")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func getGCEMetadata(path string) ([]byte, error) {
	// See https://cloud.google.com/compute/docs/storing-retrieving-metadata#default
	metadataURL := "http://metadata.google.internal/computeMetadata/v1/" + path
	req, err := http.NewRequest("GET", metadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create http request for %q: %s", metadataURL, err)
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain response to %q: %s", metadataURL, err)
	}
	return readResponseBody(resp, metadataURL)
}
