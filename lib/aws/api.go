package aws

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

// getSTSAPIResponse makes request to aws sts api with Config
// and returns temporary credentials with expiration time
//
// See https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
func getSTSAPIResponse(action string, cfg *Config, reqBuilder func(apiURL string) (*http.Request, error)) ([]byte, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html
	apiURL := fmt.Sprintf("%s?Action=%s", cfg.stsEndpoint, action)
	apiURL += "&Version=2011-06-15"
	apiURL += fmt.Sprintf("&RoleArn=%s", cfg.roleARN)
	// we have to provide unique session name for cloudtrail audit
	apiURL += "&RoleSessionName=vmagent-ec2-discovery"
	req, err := reqBuilder(apiURL)
	if err != nil {
		return nil, fmt.Errorf("cannot create signed request: %w", err)
	}
	resp, err := cfg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot perform http request to %q: %w", apiURL, err)
	}
	return readResponseBody(resp, apiURL)

}

// GetEC2APIResponse performs EC2 API request with given action.
func GetEC2APIResponse(cfg *Config, action, nextPageToken string) ([]byte, error) {
	ac, err := cfg.getFreshAPICredentials()
	if err != nil {
		return nil, err
	}
	apiURL := fmt.Sprintf("%s?Action=%s", cfg.ec2Endpoint, url.QueryEscape(action))
	if len(cfg.filters) > 0 {
		apiURL += "&" + cfg.filters
	}
	if len(nextPageToken) > 0 {
		apiURL += fmt.Sprintf("&NextToken=%s", url.QueryEscape(nextPageToken))
	}
	apiURL += "&Version=2013-10-15"
	req, err := NewSignedGetRequestWithTime(apiURL, "ec2", cfg.region, ac, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("cannot create signed request: %w", err)
	}
	resp, err := cfg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot perform http request to %q: %w", apiURL, err)
	}
	return readResponseBody(resp, apiURL)
}

func readResponseBody(resp *http.Response, apiURL string) ([]byte, error) {
	data, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot read response from %q: %w", apiURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code for %q; got %d; want %d; response body: %q",
			apiURL, resp.StatusCode, http.StatusOK, data)
	}
	return data, nil
}
