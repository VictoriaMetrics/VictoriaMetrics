package digitalocean

import (
	"reflect"
	"testing"
)

func Test_parseAPIResponse(t *testing.T) {
	type args struct {
		data []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *listDropletResponse
		wantErr bool
	}{

		{
			name: "simple parse",
			args: args{data: []byte(`{
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
}`)},
			want: &listDropletResponse{
				Droplets: []droplet{
					{
						Image: struct {
							Name string `json:"name"`
							Slug string `json:"slug"`
						}(struct {
							Name string
							Slug string
						}{Name: "14.04 x64", Slug: "ubuntu-16-04-x64"}),
						Region: struct {
							Slug string `json:"slug"`
						}(struct{ Slug string }{Slug: "nyc3"}),
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
					Pages: struct {
						Last string `json:"last,omitempty"`
						Next string `json:"next,omitempty"`
					}(struct {
						Last string
						Next string
					}{Last: "https://api.digitalocean.com/v2/droplets?page=3&per_page=1", Next: "https://api.digitalocean.com/v2/droplets?page=2&per_page=1"}),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAPIResponse(tt.args.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAPIResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAPIResponse() got = \n%v\n, \nwant \n%v\n", got, tt.want)
			}
		})
	}
}

func Test_getDroplets(t *testing.T) {
	type args struct {
		getAPIResponse func(string) ([]byte, error)
	}
	tests := []struct {
		name             string
		args             args
		wantDropletCount int
		wantErr          bool
	}{
		{
			name: "get 4 droples",
			args: args{
				func(s string) ([]byte, error) {
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
				},
			},
			wantDropletCount: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getDroplets(tt.args.getAPIResponse)
			if (err != nil) != tt.wantErr {
				t.Errorf("getDroplets() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != tt.wantDropletCount {
				t.Fatalf("unexpected droplets count: %d, want: %d, \n droplets: %v\n", len(got), tt.wantDropletCount, got)
			}

		})
	}
}
