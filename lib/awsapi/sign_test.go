package awsapi

import (
	"strings"
	"testing"
	"time"
)

func TestNewSignedRequest(t *testing.T) {
	f := func(apiURL string, authHeaderExpected string) {
		t.Helper()
		service := "ec2"
		region := "us-east-1"
		ac := &credentials{
			AccessKeyID:     strings.Repeat("fake-access-key", 2),
			SecretAccessKey: strings.Repeat("foobar", 10),
		}
		ct := time.Unix(0, 0).UTC()
		req, err := newSignedGetRequestWithTime(apiURL, service, region, ac, ct)
		if err != nil {
			t.Fatalf("error in newSignedRequest: %s", err)
		}
		authHeader := req.Header.Get("Authorization")
		if authHeader != authHeaderExpected {
			t.Fatalf("unexpected auth header;\ngot\n%s\nwant\n%s", authHeader, authHeaderExpected)
		}
	}
	f("https://ec2.amazonaws.com/?Action=DescribeRegions&Version=2013-10-15",
		"AWS4-HMAC-SHA256 Credential=fake-access-keyfake-access-key/19700101/us-east-1/ec2/aws4_request, SignedHeaders=host;x-amz-date, Signature=e6c0f635693173f83eea9f443ae364d9099c98b0f5e7b1356e7cfc9c742daea2")
}
