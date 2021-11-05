package openstack

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func Test_buildAuthRequestBody1(t *testing.T) {
	type args struct {
		sdc *SDConfig
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "empty config",
			args: args{
				sdc: &SDConfig{},
			},
			wantErr: true,
		},
		{
			name: "username password auth with domain",
			args: args{
				sdc: &SDConfig{
					Username:   "some-user",
					Password:   promauth.NewSecret("some-password"),
					DomainName: "some-domain",
				},
			},
			want: []byte(`{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"some-user","password":"some-password","domain":{"name":"some-domain"}}}},"scope":{"domain":{"name":"some-domain"}}}}`),
		},
		{
			name: "application credentials auth",
			args: args{
				sdc: &SDConfig{
					ApplicationCredentialID:     "some-id",
					ApplicationCredentialSecret: promauth.NewSecret("some-secret"),
				},
			},
			want: []byte(`{"auth":{"identity":{"methods":["application_credential"],"application_credential":{"id":"some-id","secret":"some-secret"}}}}`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildAuthRequestBody(tt.args.sdc)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildAuthRequestBody() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("buildAuthRequestBody() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getComputeEndpointURL1(t *testing.T) {
	type args struct {
		catalog      []catalogItem
		availability string
		region       string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "bad catalog data",
			args: args{
				catalog: []catalogItem{
					{
						Type:      "keystone",
						Endpoints: []endpoint{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "good private url",
			args: args{
				availability: "private",
				catalog: []catalogItem{
					{
						Type: "compute",
						Endpoints: []endpoint{
							{
								Interface: "private",
								Type:      "compute",
								URL:       "https://compute.test.local:8083/v2.1",
							},
						},
					},
					{
						Type:      "keystone",
						Endpoints: []endpoint{},
					},
				},
			},
			want: "https://compute.test.local:8083/v2.1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getComputeEndpointURL(tt.args.catalog, tt.args.availability, tt.args.region)
			if (err != nil) != tt.wantErr {
				t.Errorf("getComputeEndpointURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && !reflect.DeepEqual(got.String(), tt.want) {
				t.Errorf("getComputeEndpointURL() got = %v, want %v", got.String(), tt.want)
			}
		})
	}
}
