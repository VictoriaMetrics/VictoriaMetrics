package ec2

import (
	"testing"
	"time"
)

func TestNewSignedRequest(t *testing.T) {
	f := func(apiURL string, authHeaderExpected string) {
		t.Helper()
		service := "ec2"
		region := "us-east-1"
		accessKey := "fake-access-key"
		secretKey := "foobar"
		ct := time.Unix(0, 0).UTC()
		req, err := newSignedRequestWithTime(apiURL, service, region, accessKey, secretKey, ct)
		if err != nil {
			t.Fatalf("error in newSignedRequest: %s", err)
		}
		authHeader := req.Header.Get("Authorization")
		if authHeader != authHeaderExpected {
			t.Fatalf("unexpected auth header;\ngot\n%s\nwant\n%s", authHeader, authHeaderExpected)
		}
	}
	f("https://ec2.amazonaws.com/?Action=DescribeRegions&Version=2013-10-15",
		"AWS4-HMAC-SHA256 Credential=fake-access-key/19700101/us-east-1/ec2/aws4_request, SignedHeaders=host;x-amz-date, Signature=79dc8f54719a4c11edcd5811824a071361b3514172a3f5c903b7e279dfa6a710")
}
