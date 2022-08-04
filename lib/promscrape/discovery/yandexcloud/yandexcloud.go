package yandexcloud

import (
	"flag"
	"fmt"
	"net/url"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.yandexcloudSDCheckInterval", 30*time.Second, "Interval for checking for changes in Yandex Cloud API. "+
	"This works only if yandexcloud_sd_configs is configured in '-promscrape.config' file.")

// SDConfig is the configuration for Yandex Cloud service discovery.
type SDConfig struct {
	Service                  string              `yaml:"service"`
	YandexPassportOAuthToken *promauth.Secret    `yaml:"yandex_passport_oauth_token,omitempty"`
	APIEndpoint              string              `yaml:"api_endpoint,omitempty"`
	TLSConfig                *promauth.TLSConfig `yaml:"tls_config,omitempty"`
}

// GetLabels returns labels for Yandex Cloud according to service discover config.
func (sdc *SDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	switch sdc.Service {
	case "compute":
		return getInstancesLabels(cfg)
	default:
		return nil, fmt.Errorf("unexpected `service`: %q; only `compute` supported yet; skipping it", sdc.Service)
	}
}

func (cfg *apiConfig) getInstances(folderID string) ([]instance, error) {
	instancesURL := cfg.serviceEndpoints["compute"] + "/compute/v1/instances"
	instancesURL += "?folderId=" + url.QueryEscape(folderID)

	var instances []instance
	nextLink := instancesURL
	for {
		data, err := getAPIResponse(nextLink, cfg)
		if err != nil {
			return nil, fmt.Errorf("cannot get instances: %w", err)
		}
		ip, err := parseInstancesPage(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse instances response from %q: %w; response body: %s", nextLink, err, data)
		}
		instances = append(instances, ip.Instances...)
		if len(ip.NextPageToken) == 0 {
			return instances, nil
		}
		nextLink = instancesURL + "&pageToken=" + url.QueryEscape(ip.NextPageToken)
	}
}

func (cfg *apiConfig) getFolders(clouds []cloud) ([]folder, error) {
	foldersURL := cfg.serviceEndpoints["resource-manager"] + "/resource-manager/v1/folders"
	var folders []folder
	for _, cl := range clouds {
		cloudURL := foldersURL + "?cloudId=" + url.QueryEscape(cl.ID)
		nextLink := cloudURL
		for {
			data, err := getAPIResponse(nextLink, cfg)
			if err != nil {
				return nil, fmt.Errorf("cannot get folders: %w", err)
			}
			fp, err := parseFoldersPage(data)
			if err != nil {
				return nil, fmt.Errorf("cannot parse folders response from %q: %w; response body: %s", nextLink, err, data)
			}
			folders = append(folders, fp.Folders...)
			if len(fp.NextPageToken) == 0 {
				break
			}
			nextLink = cloudURL + "&pageToken=" + url.QueryEscape(fp.NextPageToken)
		}
	}
	return folders, nil
}

func (cfg *apiConfig) getClouds(orgs []organization) ([]cloud, error) {
	cloudsURL := cfg.serviceEndpoints["resource-manager"] + "/resource-manager/v1/clouds"
	if len(orgs) == 0 {
		orgs = append(orgs, organization{
			ID: "",
		})
	}
	var clouds []cloud
	for _, org := range orgs {
		orgURL := cloudsURL
		if org.ID != "" {
			orgURL += "?organizationId=" + url.QueryEscape(org.ID)
		}
		nextLink := orgURL
		for {
			data, err := getAPIResponse(nextLink, cfg)
			if err != nil {
				return nil, fmt.Errorf("cannot get clouds: %w", err)
			}
			cp, err := parseCloudsPage(data)
			if err != nil {
				return nil, fmt.Errorf("cannot parse clouds response from %q: %w; response body: %s", nextLink, err, data)
			}
			clouds = append(clouds, cp.Clouds...)
			if len(cp.NextPageToken) == 0 {
				break
			}
			nextLink = orgURL + "&pageToken=" + url.QueryEscape(cp.NextPageToken)
		}
	}
	return clouds, nil
}

func (cfg *apiConfig) getOrganizations() ([]organization, error) {
	orgsURL := cfg.serviceEndpoints["organization-manager"] + "/organization-manager/v1/organizations"
	var orgs []organization
	nextLink := orgsURL
	for {
		data, err := getAPIResponse(nextLink, cfg)
		if err != nil {
			return nil, fmt.Errorf("cannot get organizations: %w", err)
		}
		op, err := parseOrganizationsPage(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse organizations response from %q: %w; response body: %s", nextLink, err, data)
		}
		orgs = append(orgs, op.Organizations...)
		if len(op.NextPageToken) == 0 {
			return orgs, nil
		}
		nextLink = orgsURL + "&pageToken=" + url.QueryEscape(op.NextPageToken)
	}
}
