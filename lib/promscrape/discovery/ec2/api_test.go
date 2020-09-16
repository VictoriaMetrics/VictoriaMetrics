package ec2

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
)

// RoundTripFunc .
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

//NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func newTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: fn,
	}
}

func TestNewAPIConfig(t *testing.T) {

	t.Run("get credentials from env vars", func(t *testing.T) {
		wantKey := "key"
		wantSecret := "secret"
		_ = os.Setenv(awsAccessKeyEnv, wantKey)
		_ = os.Setenv(awsSecretKeyEnv, wantSecret)
		defer func() {
			_ = os.Unsetenv(awsAccessKeyEnv)
			_ = os.Unsetenv(awsSecretKeyEnv)
		}()
		apiCfg, err := newAPIConfig(&SDConfig{Region: "eu-west-1"}, nil)
		if err != nil {
			t.Fatalf("cannot build apiConfig")
		}
		if apiCfg.secretKey != wantSecret {
			t.Fatalf("secretKey not match, want: %s, get: %s", wantSecret, apiCfg.secretKey)
		}
		if apiCfg.accessKey != wantKey {
			t.Fatalf("accessKey not match, want: %s, get: %s", wantKey, apiCfg.accessKey)
		}
	})
	t.Run("get credentials from instance iam role", func(t *testing.T) {
		wantKey := "key"
		wantSecret := "secret"
		apiCfg, err := newAPIConfig(&SDConfig{Region: "eu-west-1"}, newTestClient(func(req *http.Request) *http.Response {
			switch req.URL.String() {
			case "http://169.254.169.254/latest/api/token":
				return &http.Response{StatusCode: 200, Body: ioutil.NopCloser(bytes.NewBufferString(``))}
			case "http://169.254.169.254/latest/meta-data/iam/security-credentials/":
				return &http.Response{StatusCode: 200, Body: ioutil.NopCloser(bytes.NewBufferString(`ec2-access`))}
			case "http://169.254.169.254/latest/meta-data/iam/security-credentials/ec2-access":
				return &http.Response{
					StatusCode: 200,
					Body: ioutil.NopCloser(bytes.NewBufferString(fmt.Sprintf(`{"Expiration": "2006-01-02T15:04:05Z",
                                                              "AccessKeyId": "%s",
                                                               "SecretAccessKey":"%s"}`, wantKey, wantSecret)))}
			}
			return nil
		}))
		if err != nil {
			t.Fatalf("cannot build apiConfig, err: %v", err)
		}
		if apiCfg.secretKey != wantSecret {
			t.Fatalf("secretKey not match, want: %s, get: %s", wantSecret, apiCfg.secretKey)
		}
		if apiCfg.accessKey != wantKey {
			t.Fatalf("accessKey not match, want: %s, get: %s", wantKey, apiCfg.accessKey)
		}

	})
	t.Run("get credentials with aws sts for arn_role", func(t *testing.T) {
		wantKey := "key"
		wantSecret := "secret"
		wantToken := "token"
		apiCfg, err := newAPIConfig(&SDConfig{
			Region:    "eu-west-1",
			RoleARN:   "iam:12412:user/role-arn",
			SecretKey: "default-secret",
			AccessKey: "default-key",
		}, newTestClient(func(req *http.Request) *http.Response {
			switch req.URL.String() {
			case "http://169.254.169.254/latest/api/token":
				return &http.Response{StatusCode: 200, Body: ioutil.NopCloser(bytes.NewBufferString(``))}
			case "http://169.254.169.254/latest/meta-data/iam/security-credentials/":
				return &http.Response{StatusCode: 404, Body: ioutil.NopCloser(bytes.NewBufferString(`not found`))}
			case "https://sts.eu-west-1.amazonaws.com/?Action=AssumeRole&Version=2011-06-15&RoleArn=iam:12412:user/role-arn&RoleSessionName=vmagent-ec2-discovery":
				return &http.Response{
					StatusCode: 200,
					Body: ioutil.NopCloser(bytes.NewBufferString(fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<root>
   <AssumeRoleResult>
      <Credentials>
         <AccessKeyId>%s</AccessKeyId>
         <Expiration>2006-01-02T15:04:05Z</Expiration>
         <SecretAccessKey>%s</SecretAccessKey>
         <SessionToken>%s</SessionToken>
      </Credentials>
   </AssumeRoleResult>
</root>`, wantKey, wantSecret, wantToken)))}
			}
			return nil
		}))
		if err != nil {
			t.Fatalf("cannot build apiConfig, err: %v", err)
		}
		if apiCfg.secretKey != wantSecret {
			t.Fatalf("secretKey not match, want: %s, get: %s", wantSecret, apiCfg.secretKey)
		}
		if apiCfg.accessKey != wantKey {
			t.Fatalf("accessKey not match, want: %s, get: %s", wantKey, apiCfg.accessKey)
		}
		if apiCfg.token != wantToken {
			t.Fatalf("want token not match, want: %s, get: %s", wantToken, apiCfg.token)

		}

	})
}
