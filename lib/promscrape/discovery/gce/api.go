package gce

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/oauth2/google"
)

type apiConfig struct {
	client       *http.Client
	apiURL       string
	project      string
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
	// See https://cloud.google.com/compute/docs/reference/rest/v1/instances/list
	apiURL := fmt.Sprintf("https://compute.googleapis.com/compute/v1/projects/%s/zones/%s/instances", sdc.Project, sdc.Zone)
	if len(sdc.Filter) > 0 {
		apiURL += fmt.Sprintf("?filter=%s", url.QueryEscape(sdc.Filter))
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
		apiURL:       apiURL,
		project:      sdc.Project,
		tagSeparator: tagSeparator,
		port:         port,
	}, nil
}
