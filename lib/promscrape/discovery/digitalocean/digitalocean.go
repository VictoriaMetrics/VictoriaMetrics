package digitalocean

import (
	"flag"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.digitaloceanSDCheckInterval", time.Minute, "Interval for checking for changes in digital ocean. "+
	"This works only if digitalocean_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#digitalocean_sd_configs for details")

// SDConfig represents service discovery config for digital ocean.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#digitalocean_sd_config
type SDConfig struct {
	Server            string                     `yaml:"server,omitempty"`
	HTTPClientConfig  promauth.HTTPClientConfig  `yaml:",inline"`
	ProxyURL          *proxy.URL                 `yaml:"proxy_url,omitempty"`
	ProxyClientConfig promauth.ProxyClientConfig `yaml:",inline"`
	Port              int                        `yaml:"port,omitempty"`
}

// GetLabels returns Digital Ocean droplet labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]*promutil.Labels, error) {
	cfg, err := getAPIConfig(sdc, baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot get API config: %w", err)
	}
	droplets, err := getDroplets(cfg.client.GetAPIResponse)
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

	Features []string      `json:"features"`
	Image    dropletImage  `json:"image"`
	SizeSlug string        `json:"size_slug"`
	Networks networks      `json:"networks"`
	Region   dropletRegion `json:"region"`
	Tags     []string      `json:"tags"`
	VpcUUID  string        `json:"vpc_uuid"`
}

type dropletImage struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type dropletRegion struct {
	Slug string `json:"slug"`
}

func (d *droplet) getIPByNet(netVersion, netType string) string {
	var dropletNetworks []network
	switch netVersion {
	case "v4":
		dropletNetworks = d.Networks.V4
	case "v6":
		dropletNetworks = d.Networks.V6
	default:
		logger.Panicf("BUG: unexpected network type: %s, want v4 or v6", netVersion)
	}
	for _, net := range dropletNetworks {
		if net.Type == netType {
			return net.IPAddress
		}
	}
	return ""
}

type networks struct {
	V4 []network `json:"v4"`
	V6 []network `json:"v6"`
}
type network struct {
	IPAddress string `json:"ip_address"`
	// private | public.
	Type string `json:"type"`
}

// https://developers.digitalocean.com/documentation/v2/#list-all-droplets
type listDropletResponse struct {
	Droplets []droplet `json:"droplets,omitempty"`
	Links    links     `json:"links,omitempty"`
}

type links struct {
	Pages linksPages `json:"pages,omitempty"`
}

type linksPages struct {
	Last string `json:"last,omitempty"`
	Next string `json:"next,omitempty"`
}

func (r *listDropletResponse) nextURLPath() (string, error) {
	if r.Links.Pages.Next == "" {
		return "", nil
	}
	u, err := url.Parse(r.Links.Pages.Next)
	if err != nil {
		return "", fmt.Errorf("cannot parse digital ocean next url: %s: %w", r.Links.Pages.Next, err)
	}
	return u.RequestURI(), nil
}

func addDropletLabels(droplets []droplet, defaultPort int) []*promutil.Labels {
	var ms []*promutil.Labels
	for _, droplet := range droplets {
		if len(droplet.Networks.V4) == 0 {
			continue
		}

		privateIPv4 := droplet.getIPByNet("v4", "private")
		publicIPv4 := droplet.getIPByNet("v4", "public")
		publicIPv6 := droplet.getIPByNet("v6", "public")

		addr := discoveryutil.JoinHostPort(publicIPv4, defaultPort)
		m := promutil.NewLabels(16)
		m.Add("__address__", addr)
		m.Add("__meta_digitalocean_droplet_id", fmt.Sprintf("%d", droplet.ID))
		m.Add("__meta_digitalocean_droplet_name", droplet.Name)
		m.Add("__meta_digitalocean_image", droplet.Image.Slug)
		m.Add("__meta_digitalocean_image_name", droplet.Image.Name)
		m.Add("__meta_digitalocean_private_ipv4", privateIPv4)
		m.Add("__meta_digitalocean_public_ipv4", publicIPv4)
		m.Add("__meta_digitalocean_public_ipv6", publicIPv6)
		m.Add("__meta_digitalocean_region", droplet.Region.Slug)
		m.Add("__meta_digitalocean_size", droplet.SizeSlug)
		m.Add("__meta_digitalocean_status", droplet.Status)
		m.Add("__meta_digitalocean_vpc", droplet.VpcUUID)
		if len(droplet.Features) > 0 {
			features := fmt.Sprintf(",%s,", strings.Join(droplet.Features, ","))
			m.Add("__meta_digitalocean_features", features)
		}
		if len(droplet.Tags) > 0 {
			tags := fmt.Sprintf(",%s,", strings.Join(droplet.Tags, ","))
			m.Add("__meta_digitalocean_tags", tags)
		}
		ms = append(ms, m)
	}
	return ms
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	v := configMap.Delete(sdc)
	if v != nil {
		cfg := v.(*apiConfig)
		cfg.client.Stop()
	}
}
