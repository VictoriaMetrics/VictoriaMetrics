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
	var zones []string
	if len(sdc.Zone) == 0 {
		// Autodetect zones for sdc.Project.
		zs, err := getZonesForProject(client, sdc.Project, sdc.Filter)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain zones for project %q: %s", sdc.Project, err)
		}
		zones = zs
	} else {
		zones = []string{sdc.Zone}
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
		project:      sdc.Project,
		filter:       sdc.Filter,
		tagSeparator: tagSeparator,
		port:         port,
	}, nil
}

func getAPIResponse(client *http.Client, apiURL, filter, pageToken string) ([]byte, error) {
	apiURL = appendNonEmptyQueryArg(apiURL, filter)
	apiURL = appendNonEmptyQueryArg(apiURL, pageToken)
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("cannot query %q: %s", apiURL, err)
	}
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

func appendNonEmptyQueryArg(apiURL, arg string) string {
	if len(arg) == 0 {
		return apiURL
	}
	prefix := "?"
	if strings.Contains(apiURL, "?") {
		prefix = "&"
	}
	return apiURL + fmt.Sprintf("%spageToken=%s", prefix, url.QueryEscape(arg))
}
