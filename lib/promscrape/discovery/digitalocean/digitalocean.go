package digitalocean

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

const listAPIPath = "v2/droplets"

// SDConfig represents service discovery config for digital ocean.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#digitalocean_sd_config
type SDConfig struct {
	Server            string                     `yaml:"server,omitempty"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          proxy.URL                  `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	Port              int                        `yaml:"port,omitempty"`
}

// GetLabels returns Digital Ocean droplet labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	droplets, err := getDroplets(cfg)
	if err != nil {
		return nil, err
	}

	return addDropletLabels(droplets, cfg.port), nil
}

// https://developers.digitalocean.com/documentation/v2/#retrieve-an-existing-droplet-by-id
type droplet struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`

	Features []string `json:"features"`
	Image    struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	} `json:"image"`
	SizeSlug string   `json:"size_slug"`
	Networks networks `json:"networks"`
	Region   struct {
		Slug string `json:"slug"`
	} `json:"region"`
	Tags    []string `json:"tags"`
	VpcUuid string   `json:"vpc_uuid"`
}

func (d *droplet) getIPByNet(netVersion, netType string) string {
	var dropletNetworks []network
	switch netVersion {
	case "v4":
		dropletNetworks = d.Networks.V4
	case "v6":
		dropletNetworks = d.Networks.V6
	default:
		logger.Fatalf("BUG, unexpected network type: %s, want v4 or v6", netVersion)
	}
	for _, net := range dropletNetworks {
		if net.Type == netType {
			return net.IpAddress
		}
	}
	return ""
}

type networks struct {
	V4 []network `json:"v4"`
	V6 []network `json:"v6"`
}
type network struct {
	IpAddress string `json:"ip_address"`
	// private | public.
	Type string `json:"type"`
}

// https://developers.digitalocean.com/documentation/v2/#list-all-droplets
type listDropletResponse struct {
	Droplets []droplet
	Links    paginationLinks
}

type paginationLinks struct {
	Last string
	Next string
}

func (r *listDropletResponse) nextURLPath() (string, error) {
	if r.Links.Next == "" {
		return "", nil
	}
	u, err := url.Parse(r.Links.Next)
	if err != nil {
		return "", fmt.Errorf("cannot parse digital ocean next url: %s, err: %s", r.Links.Next, err)
	}
	return u.Path, nil
}

func addDropletLabels(droplets []droplet, defaultPort int) []map[string]string {
	var ms []map[string]string
	for _, droplet := range droplets {
		if len(droplet.Networks.V4) == 0 {
			continue
		}

		privateIPv4 := droplet.getIPByNet("v4", "private")
		publicIPv4 := droplet.getIPByNet("v4", "public")
		publicIPv6 := droplet.getIPByNet("v6", "public")

		addr := discoveryutils.JoinHostPort(publicIPv4, defaultPort)
		m := map[string]string{
			"__address__":                      addr,
			"__meta_digitalocean_droplet_id":   fmt.Sprintf("%d", droplet.ID),
			"__meta_digitalocean_droplet_name": droplet.Name,
			"__meta_digitalocean_image":        droplet.Image.Slug,
			"__meta_digitalocean_image_name":   droplet.Image.Name,
			"__meta_digitalocean_private_ipv4": privateIPv4,
			"__meta_digitalocean_public_ipv4":  publicIPv4,
			"__meta_digitalocean_public_ipv6":  publicIPv6,
			"__meta_digitalocean_region":       droplet.Region.Slug,
			"__meta_digitalocean_size":         droplet.SizeSlug,
			"__meta_digitalocean_status":       droplet.Status,
			"__meta_digitalocean_vpc":          droplet.VpcUuid,
		}
		if len(droplet.Features) > 0 {
			features := fmt.Sprintf(",%s,", strings.Join(droplet.Features, ","))
			m["__meta_digitalocean_vpc"] = features
		}
		if len(droplet.Tags) > 0 {
			tags := fmt.Sprintf(",%s,", strings.Join(droplet.Features, ","))
			m["__meta_digitalocean_tags"] = tags
		}
		ms = append(ms, m)
	}
	return ms
}
