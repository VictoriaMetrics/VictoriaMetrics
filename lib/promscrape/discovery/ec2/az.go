package ec2

import (
	"encoding/xml"
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/awsapi"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func getAZMap(cfg *apiConfig) map[string]string {
	cfg.azMapLock.Lock()
	defer cfg.azMapLock.Unlock()

	if cfg.azMap != nil {
		return cfg.azMap
	}

	azs, err := getAvailabilityZones(cfg)
	cfg.azMap = make(map[string]string, len(azs))
	if err != nil {
		logger.Warnf("couldn't load availability zones map, so __meta_ec2_availability_zone_id label isn't set: %s", err)
		return cfg.azMap
	}
	for _, az := range azs {
		cfg.azMap[az.ZoneName] = az.ZoneID
	}
	return cfg.azMap
}

func getAvailabilityZones(cfg *apiConfig) ([]AvailabilityZone, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAvailabilityZones.html
	azFilters := awsapi.GetFiltersQueryString(cfg.azFilters)
	data, err := cfg.awsConfig.GetEC2APIResponse("DescribeAvailabilityZones", azFilters, "")
	if err != nil {
		return nil, fmt.Errorf("cannot obtain availability zones: %w", err)
	}
	azr, err := parseAvailabilityZonesResponse(data)
	if err != nil {
		return nil, fmt.Errorf("cannot parse availability zones list: %w", err)
	}
	return azr.AvailabilityZoneInfo.Items, nil
}

// AvailabilityZonesResponse represents the response for https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAvailabilityZones.html
type AvailabilityZonesResponse struct {
	AvailabilityZoneInfo AvailabilityZoneInfo `xml:"availabilityZoneInfo"`
}

// AvailabilityZoneInfo represents availabilityZoneInfo for https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeAvailabilityZones.html
type AvailabilityZoneInfo struct {
	Items []AvailabilityZone `xml:"item"`
}

// AvailabilityZone represents availabilityZone for https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AvailabilityZone.html
type AvailabilityZone struct {
	ZoneName string `xml:"zoneName"`
	ZoneID   string `xml:"zoneId"`
}

func parseAvailabilityZonesResponse(data []byte) (*AvailabilityZonesResponse, error) {
	var v AvailabilityZonesResponse
	if err := xml.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("cannot unmarshal DescribeAvailabilityZonesResponse from %q: %w", data, err)
	}
	return &v, nil
}
