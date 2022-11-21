package gce

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func getZonesForProject(client *http.Client, project string) ([]string, error) {
	// See https://cloud.google.com/compute/docs/reference/rest/v1/zones
	zonesURL := fmt.Sprintf("https://compute.googleapis.com/compute/v1/projects/%s/zones", project)
	var zones []string
	pageToken := ""
	for {
		data, err := getAPIResponse(client, zonesURL, "", pageToken)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain zones: %w", err)
		}
		zl, err := parseZoneList(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse zone list from %q: %w", zonesURL, err)
		}
		for _, z := range zl.Items {
			zones = append(zones, z.Name)
		}
		if len(zl.NextPageToken) == 0 {
			return zones, nil
		}
		pageToken = zl.NextPageToken
	}
}

// ZoneList is response to https://cloud.google.com/compute/docs/reference/rest/v1/zones/list
type ZoneList struct {
	Items         []Zone
	NextPageToken string
}

// Zone is zone from https://cloud.google.com/compute/docs/reference/rest/v1/zones/list
type Zone struct {
	Name string
}

// parseZoneList parses ZoneList from data.
func parseZoneList(data []byte) (*ZoneList, error) {
	var zl ZoneList
	if err := json.Unmarshal(data, &zl); err != nil {
		return nil, fmt.Errorf("cannot unmarshal ZoneList from %q: %w", data, err)
	}
	return &zl, nil
}
