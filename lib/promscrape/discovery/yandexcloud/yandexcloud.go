package yandexcloud

import (
	"flag"
	"fmt"
	"path"
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
	ApiEndpoint              string              `yaml:"api_endpoint,omitempty"`
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
	computeURL := *cfg.serviceEndpoints["compute"]
	computeURL.Path = path.Join(computeURL.Path, "compute", defaultAPIVersion, "instances")
	q := computeURL.Query()
	q.Set("folderId", folderID)
	computeURL.RawQuery = q.Encode()
	nextLink := computeURL.String()

	instances := make([]instance, 0)
	for {
		resp, err := getAPIResponse(nextLink, cfg)
		if err != nil {
			return nil, err
		}
		instancesPage, err := parseInstancesPage(resp)
		if err != nil {
			return nil, err
		}
		instances = append(instances, instancesPage.Instances...)
		if len(instancesPage.NextPageToken) == 0 {
			return instances, nil
		}

		q.Set("pageToken", instancesPage.NextPageToken)
		computeURL.RawQuery = q.Encode()
		nextLink = computeURL.String()
	}
}

func (cfg *apiConfig) getFolders(clouds []cloud) ([]folder, error) {
	rmURL := *cfg.serviceEndpoints["resource-manager"]
	rmURL.Path = path.Join(rmURL.Path, "resource-manager", defaultAPIVersion, "folders")
	q := rmURL.Query()

	folders := make([]folder, 0)
	for _, cl := range clouds {
		q.Set("cloudId", cl.ID)
		rmURL.RawQuery = q.Encode()

		nextLink := rmURL.String()
		for {
			resp, err := getAPIResponse(nextLink, cfg)
			if err != nil {
				return nil, err
			}

			foldersPage, err := parseFoldersPage(resp)
			if err != nil {
				return nil, err
			}

			folders = append(folders, foldersPage.Folders...)

			if len(foldersPage.NextPageToken) == 0 {
				break
			}

			q.Set("pageToken", foldersPage.NextPageToken)
			rmURL.RawQuery = q.Encode()
			nextLink = rmURL.String()
		}
	}

	return folders, nil
}

func (cfg *apiConfig) getClouds(organizations []organization) ([]cloud, error) {
	rmURL := *cfg.serviceEndpoints["resource-manager"]
	rmURL.Path = path.Join(rmURL.Path, "resource-manager", defaultAPIVersion, "clouds")
	q := rmURL.Query()

	if len(organizations) == 0 {
		organizations = append(organizations, organization{
			ID: "",
		})
	}

	clouds := make([]cloud, 0)
	for _, org := range organizations {
		if org.ID != "" {
			q.Set("organizationId", org.ID)
			rmURL.RawQuery = q.Encode()
		}

		nextLink := rmURL.String()
		for {
			resp, err := getAPIResponse(nextLink, cfg)
			if err != nil {
				return nil, err
			}

			cloudsPage, err := parseCloudsPage(resp)
			if err != nil {
				return nil, err
			}

			clouds = append(clouds, cloudsPage.Clouds...)

			if len(cloudsPage.NextPageToken) == 0 {
				break
			}

			q.Set("pageToken", cloudsPage.NextPageToken)
			rmURL.RawQuery = q.Encode()
			nextLink = rmURL.String()
		}
	}

	return clouds, nil
}

func (cfg *apiConfig) getOrganizations() ([]organization, error) {
	omURL := *cfg.serviceEndpoints["organization-manager"]
	omURL.Path = path.Join(omURL.Path, "organization-manager", defaultAPIVersion, "organizations")
	q := omURL.Query()
	nextLink := omURL.String()

	organizations := make([]organization, 0)
	for {
		resp, err := getAPIResponse(nextLink, cfg)
		if err != nil {
			return nil, err
		}

		organizationsPage, err := parseOrganizationsPage(resp)
		if err != nil {
			return nil, err
		}

		organizations = append(organizations, organizationsPage.Organizations...)

		if len(organizationsPage.NextPageToken) == 0 {
			return organizations, nil
		}

		q.Set("pageToken", organizationsPage.NextPageToken)
		omURL.RawQuery = q.Encode()
		nextLink = omURL.String()
	}
}
