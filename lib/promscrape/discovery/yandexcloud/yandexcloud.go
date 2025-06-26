package yandexcloud

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.yandexcloudSDCheckInterval", 30*time.Second, "Interval for checking for changes in Yandex Cloud API. "+
	"This works only if yandexcloud_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#yandexcloud_sd_configs for details")

// SDConfig is the configuration for Yandex Cloud service discovery.
type SDConfig struct {
	Service                  string              `yaml:"service"`
	YandexPassportOAuthToken *promauth.Secret    `yaml:"yandex_passport_oauth_token,omitempty"`
	APIEndpoint              string              `yaml:"api_endpoint,omitempty"`
	TLSConfig                *promauth.TLSConfig `yaml:"tls_config,omitempty"`
}

// GetLabels returns labels for Yandex Cloud according to service discover config.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	switch sdc.Service {
	case "compute":
		return getInstancesLabels(cfg)
	default:
		return nil, fmt.Errorf("skipping unexpected service=%q; only `compute` supported for now", sdc.Service)
	}
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	_ = configMap.Delete(sdc)
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
		var ip instancesPage
		if err := json.Unmarshal(data, &ip); err != nil {
			return nil, fmt.Errorf("cannot parse instances response from %q: %w; response body: %s", nextLink, err, data)
		}
		instances = append(instances, ip.Instances...)
		if len(ip.NextPageToken) == 0 {
			return instances, nil
		}
		nextLink = instancesURL + "&pageToken=" + url.QueryEscape(ip.NextPageToken)
	}
}

// See https://cloud.yandex.com/en-ru/docs/compute/api-ref/Instance/list
type instancesPage struct {
	Instances     []instance `json:"instances"`
	NextPageToken string     `json:"nextPageToken"`
}

type instance struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	FQDN              string             `json:"fqdn"`
	Status            string             `json:"status"`
	FolderID          string             `json:"folderId"`
	PlatformID        string             `json:"platformId"`
	Resources         resources          `json:"resources"`
	NetworkInterfaces []networkInterface `json:"networkInterfaces"`
	Labels            map[string]string  `json:"labels,omitempty"`
}

type resources struct {
	Cores        string `json:"cores"`
	CoreFraction string `json:"coreFraction"`
	Memory       string `json:"memory"`
}

type networkInterface struct {
	Index            string           `json:"index"`
	MacAddress       string           `json:"macAddress"`
	SubnetID         string           `json:"subnetId"`
	PrimaryV4Address primaryV4Address `json:"primaryV4Address"`
}

type primaryV4Address struct {
	Address     string      `json:"address"`
	OneToOneNat oneToOneNat `json:"oneToOneNat"`
	DNSRecords  []dnsRecord `json:"dnsRecords"`
}

type oneToOneNat struct {
	Address    string      `json:"address"`
	IPVersion  string      `json:"ipVersion"`
	DNSRecords []dnsRecord `json:"dnsRecords"`
}

type dnsRecord struct {
	FQDN      string `json:"fqdn"`
	DNSZoneID string `json:"dnsZoneId"`
	TTL       string `json:"ttl"`
	PTR       bool   `json:"ptr"`
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
			var fp foldersPage
			if err := json.Unmarshal(data, &fp); err != nil {
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

// See https://cloud.yandex.com/en-ru/docs/resource-manager/api-ref/Folder/list
type foldersPage struct {
	Folders       []folder `json:"folders"`
	NextPageToken string   `json:"nextPageToken"`
}

type folder struct {
	Name        string            `json:"name"`
	ID          string            `json:"id"`
	CloudID     string            `json:"cloudId"`
	Description string            `json:"description"`
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	CreatedAt   time.Time         `json:"createdAt"`
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
			var cp cloudsPage
			if err := json.Unmarshal(data, &cp); err != nil {
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

// See https://cloud.yandex.com/en-ru/docs/resource-manager/api-ref/Cloud/list
type cloudsPage struct {
	Clouds        []cloud `json:"clouds"`
	NextPageToken string  `json:"nextPageToken"`
}

type cloud struct {
	Name           string            `json:"name"`
	ID             string            `json:"id"`
	Labels         map[string]string `json:"labels"`
	OrganizationID string            `json:"organizationId"`
	Description    string            `json:"description"`
	CreatedAt      time.Time         `json:"createdAt"`
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
		var op organizationsPage
		if err := json.Unmarshal(data, &op); err != nil {
			return nil, fmt.Errorf("cannot parse organizations response from %q: %w; response body: %s", nextLink, err, data)
		}
		orgs = append(orgs, op.Organizations...)
		if len(op.NextPageToken) == 0 {
			return orgs, nil
		}
		nextLink = orgsURL + "&pageToken=" + url.QueryEscape(op.NextPageToken)
	}
}

// See https://cloud.yandex.com/en-ru/docs/organization/api-ref/Organization/list
type organizationsPage struct {
	Organizations []organization `json:"organizations"`
	NextPageToken string         `json:"nextPageToken"`
}

type organization struct {
	Name        string            `json:"name"`
	ID          string            `json:"id"`
	Labels      map[string]string `json:"labels"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"createdAt"`
}
