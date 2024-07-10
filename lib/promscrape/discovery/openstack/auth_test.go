package openstack

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
)

func TestBuildAuthRequestBody_Failure(t *testing.T) {
	f := func(sdc *SDConfig) {
		t.Helper()

		_, err := buildAuthRequestBody(sdc)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// empty config
	f(&SDConfig{})
}

func TestBuildAuthRequestBody_Success(t *testing.T) {
	f := func(sdc *SDConfig, resultExpected string) {
		t.Helper()

		result, err := buildAuthRequestBody(sdc)
		if err != nil {
			t.Fatalf("buildAuthRequestBody() error: %s", err)
		}
		if string(result) != resultExpected {
			t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	// username password auth with domain
	f(&SDConfig{
		Username:   "some-user",
		Password:   promauth.NewSecret("some-password"),
		DomainName: "some-domain",
	}, `{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"some-user","password":"some-password","domain":{"name":"some-domain"}}}},"scope":{"domain":{"name":"some-domain"}}}}`)

	// application credentials auth
	f(&SDConfig{
		ApplicationCredentialID:     "some-id",
		ApplicationCredentialSecret: promauth.NewSecret("some-secret"),
	}, `{"auth":{"identity":{"methods":["application_credential"],"application_credential":{"id":"some-id","secret":"some-secret"}}}}`)
}

func TestGetComputeEndpointURL_Failure(t *testing.T) {
	f := func(catalog []catalogItem) {
		t.Helper()

		_, err := getComputeEndpointURL(catalog, "", "")
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// bad catalog data
	catalog := []catalogItem{
		{
			Type:      "keystone",
			Endpoints: []endpoint{},
		},
	}
	f(catalog)
}

func TestGetComputeEndpointURL_Success(t *testing.T) {
	f := func(catalog []catalogItem, availability, region, resultExpected string) {
		t.Helper()

		resultURL, err := getComputeEndpointURL(catalog, availability, region)
		if err != nil {
			t.Fatalf("getComputeEndpointURL() error: %s", err)
		}

		if resultURL.String() != resultExpected {
			t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", resultURL, resultExpected)
		}
	}

	// good private url
	catalog := []catalogItem{
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
	}
	availability := "private"
	resultExpected := "https://compute.test.local:8083/v2.1"
	f(catalog, availability, "", resultExpected)
}
