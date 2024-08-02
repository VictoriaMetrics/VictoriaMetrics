package digitalocean

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func TestAddDropletLabels(t *testing.T) {
	f := func(droplets []droplet, labelssExpected []*promutils.Labels) {
		t.Helper()

		labelss := addDropletLabels(droplets, 9100)
		discoveryutils.TestEqualLabelss(t, labelss, labelssExpected)
	}

	// base labels add test
	droplets := []droplet{
		{
			ID:     15,
			Tags:   []string{"private", "test"},
			Status: "active",
			Name:   "ubuntu-1",
			Region: dropletRegion{
				Slug: "do",
			},
			Features: []string{"feature-1", "feature-2"},
			SizeSlug: "base-1",
			VpcUUID:  "vpc-1",
			Image: dropletImage{
				Name: "ubuntu",
				Slug: "18",
			},
			Networks: networks{
				V4: []network{
					{
						Type:      "public",
						IPAddress: "100.100.100.100",
					},
					{
						Type:      "private",
						IPAddress: "10.10.10.10",
					},
				},
				V6: []network{
					{
						Type:      "public",
						IPAddress: "::1",
					},
				},
			},
		},
	}
	labelssExpected := []*promutils.Labels{
		promutils.NewLabelsFromMap(map[string]string{
			"__address__":                      "100.100.100.100:9100",
			"__meta_digitalocean_droplet_id":   "15",
			"__meta_digitalocean_droplet_name": "ubuntu-1",
			"__meta_digitalocean_features":     ",feature-1,feature-2,",
			"__meta_digitalocean_image":        "18",
			"__meta_digitalocean_image_name":   "ubuntu",
			"__meta_digitalocean_private_ipv4": "10.10.10.10",
			"__meta_digitalocean_public_ipv4":  "100.100.100.100",
			"__meta_digitalocean_public_ipv6":  "::1",
			"__meta_digitalocean_region":       "do",
			"__meta_digitalocean_size":         "base-1",
			"__meta_digitalocean_status":       "active",
			"__meta_digitalocean_tags":         ",private,test,",
			"__meta_digitalocean_vpc":          "vpc-1",
		}),
	}
	f(droplets, labelssExpected)
}
