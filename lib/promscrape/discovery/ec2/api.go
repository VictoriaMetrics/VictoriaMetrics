package ec2

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

const (
	awsAccessKeyEnv = "AWS_ACCESS_KEY_ID"
	awsSecretKeyEnv = "AWS_SECRET_ACCESS_KEY"
)

type apiConfig struct {
	endpoint string
	region   string
	roleARN  string
	filters  string
	port     int
	// this keys  needed,
	// when we are using temporary credentials
	defaultAccessKey string
	defaultSecretKey string
	apiCredentials   atomic.Value
}

// apiCredentials represents aws api credentials
type apiCredentials struct {
	AccessKeyID     string    `json:"AccessKeyId"`
	SecretAccessKey string    `json:"SecretAccessKey"`
	Token           string    `json:"Token"`
	Expiration      time.Time `json:"Expiration"`
}

// stsCredentialsResponse - represents aws sts AssumeRole response
// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
type stsCredentialsResponse struct {
	AssumeRoleResult struct {
		Credentials struct {
			AccessKeyID     string    `xml:"AccessKeyId"`
			SecretAccessKey string    `xml:"SecretAccessKey"`
			SessionToken    string    `xml:"SessionToken"`
			Expiration      time.Time `xml:"Expiration"`
		}
	}
}

var configMap = discoveryutils.NewConfigMap()

func getAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) {
		return newAPIConfig(sdc)
	})
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
	filters := getFiltersQueryString(sdc.Filters)
	port := 80
	if sdc.Port != nil {
		port = *sdc.Port
	}
	cfg := &apiConfig{
		endpoint: sdc.Endpoint,
		region:   region,
		roleARN:  sdc.RoleARN,
		filters:  filters,
		port:     port,
	}
	// explicitly set credentials has priority over env variables
	cfg.defaultAccessKey = os.Getenv(awsAccessKeyEnv)
	cfg.defaultSecretKey = os.Getenv(awsSecretKeyEnv)
	if len(sdc.AccessKey) > 0 {
		cfg.defaultAccessKey = sdc.AccessKey
	}
	if len(sdc.SecretKey) > 0 {
		cfg.defaultSecretKey = sdc.SecretKey
	}

	// fast return if credentials are set and there is no roleARN
	if len(cfg.defaultAccessKey) > 0 && len(cfg.defaultSecretKey) > 0 && len(sdc.RoleARN) == 0 {
		cfg.apiCredentials.Store(&apiCredentials{
			AccessKeyID:     cfg.defaultAccessKey,
			SecretAccessKey: cfg.defaultSecretKey,
		})
		return cfg, nil
	}

	if err := cfg.refreshAPICredentials(); err != nil {
		return nil, err
	}

	return cfg, nil
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

// refreshAPICredentials updates expired tokens.
func (cfg *apiConfig) refreshAPICredentials() error {

	newAPICredentials := &apiCredentials{
		AccessKeyID:     cfg.defaultAccessKey,
		SecretAccessKey: cfg.defaultSecretKey,
	}

	// we need instance credentials
	// if dont have key and secret
	if len(newAPICredentials.AccessKeyID) == 0 && len(newAPICredentials.SecretAccessKey) == 0 {

		ac, err := getInstanceRoleCredentials()
		if err != nil {
			return err
		}
		newAPICredentials.Token = ac.Token
		newAPICredentials.SecretAccessKey = ac.SecretAccessKey
		newAPICredentials.AccessKeyID = ac.AccessKeyID
		newAPICredentials.Expiration = ac.Expiration

	}

	// read credentials from sts api, if role_arn is defined
	if cfg.roleARN != "" {

		ac, err := getRoleARNCredentials(cfg.region, cfg.endpoint, cfg.roleARN, newAPICredentials)
		if err != nil {
			return fmt.Errorf("cannot get credentials for role_arn: %s: %w", cfg.roleARN, err)
		}
		if newAPICredentials.Expiration.IsZero() || ac.Expiration.Before(newAPICredentials.Expiration) {
			newAPICredentials.Expiration = ac.Expiration
		}
		newAPICredentials.AccessKeyID = ac.AccessKeyID
		newAPICredentials.SecretAccessKey = ac.SecretAccessKey
		newAPICredentials.Token = ac.Token
	}

	if len(newAPICredentials.AccessKeyID) == 0 {
		return fmt.Errorf("missing `access_key`, you can set it with %s env var, "+
			"directly at `ec2_sd_config` as `access_key` or use instance iam role", awsAccessKeyEnv)
	}
	if len(newAPICredentials.SecretAccessKey) == 0 {
		return fmt.Errorf("missing `secret_key`, you can set it with %s env var,"+
			"directly at `ec2_sd_config` as `secret_key` or use instance iam role", awsSecretKeyEnv)
	}

	cfg.apiCredentials.Store(newAPICredentials)

	return nil
}

// credentialsExpired - checks if tokens refresh is needed
func (cfg *apiConfig) credentialsExpired() bool {
	ac := cfg.credentials()
	return !ac.Expiration.IsZero() && time.Since(ac.Expiration) > -5*time.Second
}

// credentials - returns current api credentials
func (cfg *apiConfig) credentials() *apiCredentials {
	return cfg.apiCredentials.Load().(*apiCredentials)
}

// getInstanceRoleCredentials makes request to local ec2 instance metadata service
// and tries to retrieve credentials from assigned iam role.
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
func getInstanceRoleCredentials() (*apiCredentials, error) {
	instanceRoleName, err := getMetadataByPath("meta-data/iam/security-credentials/")
	if err != nil {
		return nil, fmt.Errorf("cannot get instanceRoleName: %w", err)
	}

	credentials, err := getMetadataByPath("meta-data/iam/security-credentials/" + string(instanceRoleName))
	if err != nil {
		return nil, fmt.Errorf("cannot get instanceCredentials with role %s, error: %w", instanceRoleName, err)
	}
	ac := &apiCredentials{}
	if err := json.Unmarshal(credentials, ac); err != nil {
		return nil, fmt.Errorf("cannot parse instance metadata role response : %w", err)
	}
	return ac, nil
}

// IdentityDocument is identity document returned from AWS metadata server.
//
// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
type IdentityDocument struct {
	Region string
}

// getMetadataByPath returns instance metadata by url path
func getMetadataByPath(apiPath string) ([]byte, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html

	// Obtain session token
	sessionTokenURL := "http://169.254.169.254/latest/api/token"
	req, err := http.NewRequest("PUT", sessionTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request for IMDSv2 session token at url %q: %w", sessionTokenURL, err)
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")
	resp, err := discoveryutils.GetHTTPClient().Do(req)
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
	resp, err = discoveryutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain response for %q: %w", apiURL, err)
	}
	return readResponseBody(resp, apiURL)
}

// getRoleARNCredentials - retrieves credentials from aws sts with role_arn
func getRoleARNCredentials(region, endpoint, roleARN string, credentials *apiCredentials) (*apiCredentials, error) {

	data, err := getSTSAPIResponse(region, endpoint, roleARN, credentials)
	if err != nil {
		return nil, err
	}
	scr := &stsCredentialsResponse{}
	if err := xml.Unmarshal(data, scr); err != nil {
		return nil, fmt.Errorf("cannot parse sts api response: %w", err)
	}
	return &apiCredentials{
		AccessKeyID:     scr.AssumeRoleResult.Credentials.AccessKeyID,
		SecretAccessKey: scr.AssumeRoleResult.Credentials.SecretAccessKey,
		Token:           scr.AssumeRoleResult.Credentials.SessionToken,
		Expiration:      scr.AssumeRoleResult.Credentials.Expiration,
	}, nil

}

// buildAPIEndpoint - creates endpoint for aws api access
func buildAPIEndpoint(region, cfgEndpoint, service string) string {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html
	apiEndpoint := fmt.Sprintf("https://%s.%s.amazonaws.com/", service, region)
	if len(cfgEndpoint) > 0 {
		apiEndpoint = cfgEndpoint
		// endpoint may contain only hostname. Convert it to proper url then.
		if !strings.Contains(apiEndpoint, "://") {
			apiEndpoint = "https://" + apiEndpoint
		}
		if !strings.HasSuffix(apiEndpoint, "/") {
			apiEndpoint += "/"
		}
	}
	return apiEndpoint
}

// getSTSAPIResponse makes request to aws sts api with role_arn
// and returns temporary credentials with expiration time
// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
func getSTSAPIResponse(region, endpoint, roleARN string, credentials *apiCredentials) ([]byte, error) {
	endpoint = buildAPIEndpoint(region, endpoint, "sts")
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html
	apiURL := fmt.Sprintf("%s?Action=%s", endpoint, "AssumeRole")
	apiURL += "&Version=2011-06-15"
	apiURL += fmt.Sprintf("&RoleArn=%s", roleARN)
	// we have to provide unique session name for cloudtrail audit
	apiURL += "&RoleSessionName=vmagent-ec2-discovery"
	req, err := newSignedRequest(apiURL, "sts", region, credentials)
	if err != nil {
		return nil, fmt.Errorf("cannot create signed request: %w", err)
	}
	resp, err := discoveryutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot perform http request to %q: %w", apiURL, err)
	}
	return readResponseBody(resp, apiURL)

}

// getEC2APIResponse lists ec2 instances with given action
func getEC2APIResponse(cfg *apiConfig, endpoint, action, nextPageToken string) ([]byte, error) {

	// refresh api credentials if needed
	if cfg.credentialsExpired() {
		if err := cfg.refreshAPICredentials(); err != nil {
			return nil, fmt.Errorf("failed to update expired aws credentials: %w", err)
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
	req, err := newSignedRequest(apiURL, "ec2", cfg.region, cfg.credentials())
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
