package linode

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

var configMap = discoveryutil.NewConfigMap()

type apiConfig struct {
	client       *discoveryutil.Client
	port         int
	tagSeparator string
	region       string
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}

	apiServer := sdc.Server
	if apiServer == "" {
		apiServer = "https://api.linode.com"
	}
	if !strings.Contains(apiServer, "://") {
		scheme := "http"
		if sdc.HTTPClientConfig.TLSConfig != nil {
			scheme = "https"
		}
		apiServer = scheme + "://" + apiServer
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}
	client, err := discoveryutil.NewClient(apiServer, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", apiServer, err)
	}

	port := sdc.Port
	if port == 0 {
		port = 80
	}
	tagSeparator := sdc.TagSeparator
	if tagSeparator == "" {
		tagSeparator = ","
	}
	return &apiConfig{
		client:       client,
		port:         port,
		tagSeparator: tagSeparator,
		region:       sdc.Region,
	}, nil
}

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

// Linode list API types. See https://www.linode.com/docs/api/

type instance struct {
	ID         int      `json:"id"`
	Label      string   `json:"label"`
	Group      string   `json:"group"`
	Status     string   `json:"status"`
	Type       string   `json:"type"`
	IPv4       []string `json:"ipv4"`
	IPv6       string   `json:"ipv6"`
	Image      string   `json:"image"`
	Region     string   `json:"region"`
	Specs      specs    `json:"specs"`
	Backups    backups  `json:"backups"`
	Hypervisor string   `json:"hypervisor"`
	Tags       []string `json:"tags"`
}

type specs struct {
	Disk     int `json:"disk"`
	Memory   int `json:"memory"`
	VCPUs    int `json:"vcpus"`
	GPUs     int `json:"gpus"`
	Transfer int `json:"transfer"`
}

type backups struct {
	Enabled bool `json:"enabled"`
}

type ipAddress struct {
	Address string `json:"address"`
	Public  bool   `json:"public"`
	RDNS    string `json:"rdns"`
}

type ipv6Range struct {
	Range       string `json:"range"`
	Prefix      int    `json:"prefix"`
	RouteTarget string `json:"route_target"`
}

type listResponse struct {
	Data  json.RawMessage `json:"data"`
	Page  int             `json:"page"`
	Pages int             `json:"pages"`
}

const (
	instancesAPIPath  = "/v4/linode/instances"
	ipAddressesPath   = "/v4/networking/ips"
	ipv6RangesAPIPath = "/v4/networking/ipv6/ranges"
	pageSize          = 500
)

func getInstances(cfg *apiConfig) ([]instance, error) {
	var instances []instance
	err := listAllPages(cfg, instancesAPIPath, func(data json.RawMessage) error {
		var page []instance
		if err := json.Unmarshal(data, &page); err != nil {
			return fmt.Errorf("cannot parse linode instances: %w", err)
		}
		instances = append(instances, page...)
		return nil
	})
	return instances, err
}

func getIPAddresses(cfg *apiConfig) ([]ipAddress, error) {
	var ips []ipAddress
	err := listAllPages(cfg, ipAddressesPath, func(data json.RawMessage) error {
		var page []ipAddress
		if err := json.Unmarshal(data, &page); err != nil {
			return fmt.Errorf("cannot parse linode ip addresses: %w", err)
		}
		ips = append(ips, page...)
		return nil
	})
	return ips, err
}

func getIPv6Ranges(cfg *apiConfig) ([]ipv6Range, error) {
	var ranges []ipv6Range
	err := listAllPages(cfg, ipv6RangesAPIPath, func(data json.RawMessage) error {
		var page []ipv6Range
		if err := json.Unmarshal(data, &page); err != nil {
			return fmt.Errorf("cannot parse linode ipv6 ranges: %w", err)
		}
		ranges = append(ranges, page...)
		return nil
	})
	return ranges, err
}

func listAllPages(cfg *apiConfig, apiPath string, consume func(json.RawMessage) error) error {
	page := 1
	for {
		path := fmt.Sprintf("%s?page=%d&page_size=%d", apiPath, page, pageSize)
		data, err := cfg.client.GetAPIResponseWithReqParams(path, cfg.regionFilterHeader)
		if err != nil {
			return fmt.Errorf("cannot query linode api %q: %w", path, err)
		}
		var resp listResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return fmt.Errorf("cannot parse linode api response from %q: %w; data=%q", path, err, data)
		}
		if err := consume(resp.Data); err != nil {
			return err
		}
		if resp.Pages == 0 || page >= resp.Pages {
			return nil
		}
		page++
	}
}

func (cfg *apiConfig) regionFilterHeader(req *http.Request) {
	if cfg.region == "" {
		return
	}
	// Same filter as Prometheus linode_sd: https://www.linode.com/docs/api/#filtering-and-sorting
	req.Header.Set("X-Filter", fmt.Sprintf(`{"region": "%s"}`, cfg.region))
}
