package yandexcloud

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type endpoint struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

type endpoints struct {
	Endpoints []endpoint `json:"endpoints"`
}

// See https://cloud.yandex.com/en-ru/docs/api-design-guide/concepts/endpoints
func parseEndpoints(data []byte) (*endpoints, error) {
	var endpointsResponse endpoints
	if err := json.Unmarshal(data, &endpointsResponse); err != nil {
		return nil, fmt.Errorf("cannot parse endpoints list: %w", err)
	}

	if endpointsResponse.Endpoints == nil {
		return nil, errors.New("yandex cloud API endpoints list is empty")
	}

	return &endpointsResponse, nil
}

type organization struct {
	Name        string            `json:"name"`
	ID          string            `json:"id"`
	Labels      map[string]string `json:"labels"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	CreatedAt   time.Time         `json:"createdAt"`
}

type organizationsPage struct {
	Organizations []organization `json:"organizations"`
	NextPageToken string         `json:"nextPageToken"`
}

// See https://cloud.yandex.com/en-ru/docs/organization/api-ref/Organization/list
func parseOrganizationsPage(data []byte) (*organizationsPage, error) {
	var page organizationsPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("cannot parse organizations page: %w", err)
	}

	if page.Organizations == nil {
		page.Organizations = make([]organization, 0)
	}

	return &page, nil
}

type cloud struct {
	Name           string            `json:"name"`
	ID             string            `json:"id"`
	Labels         map[string]string `json:"labels"`
	OrganizationId string            `json:"organizationId"`
	Description    string            `json:"description"`
	CreatedAt      time.Time         `json:"createdAt"`
}

type cloudsPage struct {
	Clouds        []cloud `json:"clouds"`
	NextPageToken string  `json:"nextPageToken"`
}

// See https://cloud.yandex.com/en-ru/docs/resource-manager/api-ref/Cloud/list
func parseCloudsPage(data []byte) (*cloudsPage, error) {
	var page cloudsPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("cannot parse clouds page: %w", err)
	}

	if page.Clouds == nil {
		page.Clouds = make([]cloud, 0)
	}

	return &page, nil
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

type foldersPage struct {
	Folders       []folder `json:"folders"`
	NextPageToken string   `json:"nextPageToken"`
}

// See https://cloud.yandex.com/en-ru/docs/resource-manager/api-ref/Folder/list
func parseFoldersPage(data []byte) (*foldersPage, error) {
	var page foldersPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("cannot parse folders page: %w", err)
	}

	if page.Folders == nil {
		page.Folders = make([]folder, 0)
	}

	return &page, nil
}

type dnsRecord struct {
	FQDN      string `json:"fqdn"`
	DNSZoneID string `json:"dnsZoneId"`
	TTL       string `json:"ttl"`
	PTR       bool   `json:"ptr"`
}

type oneToOneNat struct {
	Address    string      `json:"address"`
	IPVersion  string      `json:"ipVersion"`
	DnsRecords []dnsRecord `json:"dnsRecords"`
}

type primaryV4Address struct {
	Address     string      `json:"address"`
	OneToOneNat oneToOneNat `json:"oneToOneNat"`
	DnsRecords  []dnsRecord `json:"dnsRecords"`
}

type networkInterface struct {
	Index            string           `json:"index"`
	MacAddress       string           `json:"macAddress"`
	SubnetId         string           `json:"subnetId"`
	PrimaryV4Address primaryV4Address `json:"primaryV4Address"`
}

type resources struct {
	Cores        string `json:"cores"`
	CoreFraction string `json:"coreFraction"`
	Memory       string `json:"memory"`
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

type instancesPage struct {
	Instances     []instance `json:"instances"`
	NextPageToken string     `json:"nextPageToken"`
}

// See https://cloud.yandex.com/en-ru/docs/compute/api-ref/Instance/list
func parseInstancesPage(data []byte) (*instancesPage, error) {
	var page instancesPage
	if err := json.Unmarshal(data, &page); err != nil {
		return nil, fmt.Errorf("cannot parse instances page: %w", err)
	}

	if page.Instances == nil {
		page.Instances = make([]instance, 0)
	}

	return &page, nil
}
