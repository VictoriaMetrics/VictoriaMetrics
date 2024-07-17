package digitalocean

import (
	"reflect"
	"testing"
)

func TestParseAPIResponse(t *testing.T) {
	f := func(data string, responseExpected *listDropletResponse) {
		t.Helper()

		response, err := parseAPIResponse([]byte(data))
		if err != nil {
			t.Fatalf("unexpected parseAPIResponse() error: %s", err)
		}
		if !reflect.DeepEqual(response, responseExpected) {
			t.Fatalf("unexpected response\ngot\n%v\nwant\n%v", response, responseExpected)
		}
	}

	data := `
{
  "droplets": [
    {
      "id": 3164444,
      "name": "example.com",
      "memory": 1024,
      "vcpus": 1,
      "status": "active",
      "kernel": {
        "id": 2233,
        "name": "Ubuntu 14.04 x64 vmlinuz-3.13.0-37-generic",
        "version": "3.13.0-37-generic"
      },
      "features": [
        "backups",
        "ipv6",
        "virtio"
      ],
      "snapshot_ids": [],
      "image": {
        "id": 6918990,
        "name": "14.04 x64",
        "distribution": "Ubuntu",
        "slug": "ubuntu-16-04-x64",
        "public": true,
        "regions": [
          "nyc1"
        ]
      },
      "size_slug": "s-1vcpu-1gb",
      "networks": {
        "v4": [
          {
            "ip_address": "104.236.32.182",
            "netmask": "255.255.192.0",
            "gateway": "104.236.0.1",
            "type": "public"
          }
        ],
        "v6": [
          {
            "ip_address": "2604:A880:0800:0010:0000:0000:02DD:4001",
            "netmask": 64,
            "gateway": "2604:A880:0800:0010:0000:0000:0000:0001",
            "type": "public"
          }
        ]
      },
      "region": {
        "name": "New York 3",
        "slug": "nyc3",
        "features": [
          "private_networking",
          "backups",
          "ipv6"
        ]
      },
      "tags": [
        "tag1",
        "tag2"
      ],
      "vpc_uuid": "f9b0769c-e118-42fb-a0c4-fed15ef69662"
    }
  ],
  "links": {
    "pages": {
      "last": "https://api.digitalocean.com/v2/droplets?page=3&per_page=1",
      "next": "https://api.digitalocean.com/v2/droplets?page=2&per_page=1"
    }
  }
}`

	responseExpected := &listDropletResponse{
		Droplets: []droplet{
			{
				Image: dropletImage{
					Name: "14.04 x64",
					Slug: "ubuntu-16-04-x64",
				},
				Region: dropletRegion{
					Slug: "nyc3",
				},
				Networks: networks{
					V6: []network{
						{
							IPAddress: "2604:A880:0800:0010:0000:0000:02DD:4001",
							Type:      "public",
						},
					},
					V4: []network{
						{
							IPAddress: "104.236.32.182",
							Type:      "public",
						},
					},
				},
				SizeSlug: "s-1vcpu-1gb",
				Features: []string{"backups", "ipv6", "virtio"},
				Tags:     []string{"tag1", "tag2"},
				Status:   "active",
				Name:     "example.com",
				ID:       3164444,
				VpcUUID:  "f9b0769c-e118-42fb-a0c4-fed15ef69662",
			},
		},
		Links: links{
			Pages: linksPages{
				Last: "https://api.digitalocean.com/v2/droplets?page=3&per_page=1",
				Next: "https://api.digitalocean.com/v2/droplets?page=2&per_page=1",
			},
		},
	}
	f(data, responseExpected)
}

func TestGetDroplets(t *testing.T) {
	f := func(getAPIResponse func(string) ([]byte, error), expectedDropletCount int) {
		t.Helper()

		resp, err := getDroplets(getAPIResponse)
		if err != nil {
			t.Fatalf("getDroplets() error: %s", err)
		}
		if len(resp) != expectedDropletCount {
			t.Fatalf("unexpected droplets count; got %d; want %d\ndroplets:\n%v", len(resp), expectedDropletCount, resp)
		}
	}

	getAPIResponse := func(s string) ([]byte, error) {
		var resp []byte
		switch s {
		case dropletsAPIPath:
			// return next
			resp = []byte(`{ "droplets": [
    {
      "id": 3164444,
      "name": "example.com",
      "status": "active",
      "image": {
        "id": 6918990,
        "name": "14.04 x64",
        "distribution": "Ubuntu",
        "slug": "ubuntu-16-04-x64",
        "public": true,
        "regions": [
          "nyc1"
        ]
      },
      "size_slug": "s-1vcpu-1gb",
      "networks": {
        "v4": [
          {
            "ip_address": "104.236.32.182",
            "netmask": "255.255.192.0",
            "gateway": "104.236.0.1",
            "type": "public"
          }
        ]
      },
      "region": {
        "name": "New York 3",
        "slug": "nyc3"
      },
      "tags": [
        "tag1",
        "tag2"
      ],
      "vpc_uuid": "f9b0769c-e118-42fb-a0c4-fed15ef69662"
    },
    {
      "id": 3164444,
      "name": "example.com",
      "status": "active",
      "image": {
        "id": 6918990,
        "name": "14.04 x64",
        "distribution": "Ubuntu",
        "slug": "ubuntu-16-04-x64"
      },
      "size_slug": "s-1vcpu-1gb",
      "networks": {
        "v4": [
          {
            "ip_address": "104.236.32.183",
            "netmask": "255.255.192.0",
            "gateway": "104.236.0.1",
            "type": "public"
          }
        ]
      },
      "region": {
        "name": "New York 3",
        "slug": "nyc3"
      },
      "vpc_uuid": "f9b0769c-e118-42fb-a0c4-fed15ef69662"
    },
    {
      "id": 3164444,
      "name": "example.com",
      "status": "active",
      "image": {
        "id": 6918990,
        "name": "14.04 x64",
        "distribution": "Ubuntu",
        "slug": "ubuntu-16-04-x64"
      },
      "size_slug": "s-1vcpu-1gb",
      "networks": {
        "v4": [
          {
            "ip_address": "104.236.32.183",
            "netmask": "255.255.192.0",
            "gateway": "104.236.0.1",
            "type": "public"
          }
        ]
      },
      "region": {
        "name": "New York 3",
        "slug": "nyc3"
      },
      "vpc_uuid": "f9b0769c-e118-42fb-a0c4-fed15ef69662"
    }
  ],
  "links": {
    "pages": {
      "last": "https://api.digitalocean.com/v2/droplets?page=3&per_page=1",
      "next": "https://api.digitalocean.com/v2/droplets?page=2&per_page=1"
    }
  }
}`)
		default:
			// return with empty next
			resp = []byte(`{ "droplets": [
    {
      "id": 3164444,
      "name": "example.com",
      "status": "active",
      "image": {
        "id": 6918990,
        "name": "14.04 x64",
        "distribution": "Ubuntu",
        "slug": "ubuntu-16-04-x64"
      },
      "size_slug": "s-1vcpu-1gb",
      "networks": {
        "v4": [
          {
            "ip_address": "104.236.32.183",
            "netmask": "255.255.192.0",
            "gateway": "104.236.0.1",
            "type": "public"
          }
        ]
      },
      "region": {
        "name": "New York 3",
        "slug": "nyc3"
      },
      "vpc_uuid": "f9b0769c-e118-42fb-a0c4-fed15ef69662"
    },
    {
      "id": 3164444,
      "name": "example.com",
      "status": "active",
      "image": {
        "id": 6918990,
        "name": "14.04 x64",
        "distribution": "Ubuntu",
        "slug": "ubuntu-16-04-x64"
      },
      "size_slug": "s-1vcpu-1gb",
      "networks": {
        "v4": [
          {
            "ip_address": "104.236.32.183",
            "netmask": "255.255.192.0",
            "gateway": "104.236.0.1",
            "type": "public"
          }
        ]
      },
      "region": {
        "name": "New York 3",
        "slug": "nyc3"
      },
      "vpc_uuid": "f9b0769c-e118-42fb-a0c4-fed15ef69662"
    }
  ]
}`)
		}
		return resp, nil
	}
	f(getAPIResponse, 5)
}
