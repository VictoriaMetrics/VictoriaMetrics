package ec2

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

type apiConfig struct {
	endpoint  string
	region    string
	accessKey string
	secretKey string
	filters   string
	port      int
}

var configMap = discoveryutils.NewConfigMap()

func getAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	region := sdc.Region
	if len(region) == 0 {
		r, err := getDefaultRegion()
		if err != nil {
			return nil, fmt.Errorf("cannot determine default ec2 region; probably, `region` param in `ec2_sd_configs` is missing; the error: %w", err)
		}
		region = r
	}
	accessKey := sdc.AccessKey
	if len(accessKey) == 0 {
		accessKey = os.Getenv("AWS_ACCESS_KEY_ID")
		if len(accessKey) == 0 {
			return nil, fmt.Errorf("missing `access_key` in AWS_ACCESS_KEY_ID env var; probably, `access_key` must be set in `ec2_sd_config`?")
		}
	}
	secretKey := sdc.SecretKey
	if len(secretKey) == 0 {
		secretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
		if len(secretKey) == 0 {
			return nil, fmt.Errorf("miising `secret_key` in AWS_SECRET_ACCESS_KEY env var; probably, `secret_key` must be set in `ec2_sd_config`?")
		}
	}
	filters := getFiltersQueryString(sdc.Filters)
	port := 80
	if sdc.Port != nil {
		port = *sdc.Port
	}
	return &apiConfig{
		endpoint:  sdc.Endpoint,
		region:    region,
		accessKey: accessKey,
		secretKey: secretKey,
		filters:   filters,
		port:      port,
	}, nil
}

func getFiltersQueryString(filters []Filter) string {
	// See how to build filters query string at examples at https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
	var args []string
	for i, f := range filters {
		args = append(args, fmt.Sprintf("Filter.%d.Name=%s", i+1, url.QueryEscape(f.Name)))
		for j, v := range f.Values {
			args = append(args, fmt.Sprintf("Filter.%d.Value.%d=%s", i+1, j+1, url.QueryEscape(v)))
		}
	}
	return strings.Join(args, "&")
}

func getDefaultRegion() (string, error) {
	data, err := getMetadataByPath("dynamic/instance-identity/document")
	if err != nil {
		return "", err
	}
	var id IdentityDocument
	if err := json.Unmarshal(data, &id); err != nil {
		return "", fmt.Errorf("cannot parse identity document: %w", err)
	}
	return id.Region, nil
}

// IdentityDocument is identity document returned from AWS metadata server.
//
// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
type IdentityDocument struct {
	Region string
}

func getMetadataByPath(apiPath string) ([]byte, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html

	client := discoveryutils.GetHTTPClient()

	// Obtain session token
	sessionTokenURL := "http://169.254.169.254/latest/api/token"
	req, err := http.NewRequest("PUT", sessionTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request for IMDSv2 session token at url %q: %w", sessionTokenURL, err)
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain IMDSv2 session token from %q; probably, `region` is missing in `ec2_sd_config`; error: %w", sessionTokenURL, err)
	}
	token, err := readResponseBody(resp, sessionTokenURL)
	if err != nil {
		return nil, fmt.Errorf("cannot read IMDSv2 session token from %q; probably, `region` is missing in `ec2_sd_config`; error: %w", sessionTokenURL, err)
	}

	// Use session token in the request.
	apiURL := "http://169.254.169.254/latest/" + apiPath
	req, err = http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request to %q: %w", apiURL, err)
	}
	req.Header.Set("X-aws-ec2-metadata-token", string(token))
	resp, err = client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain response for %q: %w", apiURL, err)
	}
	return readResponseBody(resp, apiURL)
}

func getAPIResponse(cfg *apiConfig, action, nextPageToken string) ([]byte, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html
	endpoint := fmt.Sprintf("https://ec2.%s.amazonaws.com/", cfg.region)
	if len(cfg.endpoint) > 0 {
		endpoint = cfg.endpoint
		// endpoint may contain only hostname. Convert it to proper url then.
		if !strings.Contains(endpoint, "://") {
			endpoint = "https://" + endpoint
		}
		if !strings.HasSuffix(endpoint, "/") {
			endpoint += "/"
		}
	}
	apiURL := fmt.Sprintf("%s?Action=%s", endpoint, url.QueryEscape(action))
	if len(cfg.filters) > 0 {
		apiURL += "&" + cfg.filters
	}
	if len(nextPageToken) > 0 {
		apiURL += fmt.Sprintf("&NextToken=%s", url.QueryEscape(nextPageToken))
	}
	apiURL += "&Version=2013-10-15"
	req, err := newSignedRequest(apiURL, "ec2", cfg.region, cfg.accessKey, cfg.secretKey)
	if err != nil {
		return nil, fmt.Errorf("cannot create signed request: %w", err)
	}
	resp, err := discoveryutils.GetHTTPClient().Do(req)
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
