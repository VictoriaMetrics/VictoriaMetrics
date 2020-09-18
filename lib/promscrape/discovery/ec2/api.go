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
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

const (
	awsAccessKeyEnv = "AWS_ACCESS_KEY_ID"
	awsSecretKeyEnv = "AWS_SECRET_ACCESS_KEY"
)

type apiConfig struct {
	client          *http.Client
	endpoint        string
	region          string
	accessKey       string
	secretKey       string
	token           string
	roleARN         string
	filters         string
	port            int
	tokenExpiration time.Time
	// this keys  needed,
	// when we are using token refresh
	defaultAccessKey string
	defaultSecretKey string
}

// InstanceRoleCredentialsResponse - represent instance metadata credentials response
// in json format.
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
type InstanceRoleCredentialsResponse struct {
	Code            string
	AccessKeyId     string
	SecretAccessKey string
	Expiration      string
	Token           string
}

// StsCredentialsResponse - represents aws sts AssumeRole response
// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
type StsCredentialsResponse struct {
	AssumeRoleResult struct {
		Credentials struct {
			AccessKeyId     string `xml:"AccessKeyId"`
			SecretAccessKey string `xml:"SecretAccessKey"`
			SessionToken    string `xml:"SessionToken"`
			Expiration      string `xml:"Expiration"`
		}
	}
}

var configMap = discoveryutils.NewConfigMap()

func getAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, discoveryutils.GetHTTPClient()) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, client *http.Client) (*apiConfig, error) {
	region := sdc.Region
	if len(region) == 0 {
		r, err := getDefaultRegion(client)
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
		client:   client,
		region:   region,
		filters:  filters,
		port:     port,
		endpoint: sdc.Endpoint,
		roleARN:  sdc.RoleARN,
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
		cfg.accessKey = cfg.defaultAccessKey
		cfg.secretKey = cfg.defaultSecretKey
		return cfg, nil
	}

	if err := cfg.refreshAPIConfig(); err != nil {
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

func getDefaultRegion(client *http.Client) (string, error) {
	data, err := getMetadataByPath("dynamic/instance-identity/document", client)
	if err != nil {
		return "", err
	}
	var id IdentityDocument
	if err := json.Unmarshal(data, &id); err != nil {
		return "", fmt.Errorf("cannot parse identity document: %w", err)
	}
	return id.Region, nil
}

// refreshAPIConfig updates expired tokens.
// It fetches api keys from instance iam role
// and sts api, if role_arn is defined.
func (a *apiConfig) refreshAPIConfig() error {

	// reset credential values to default
	// or config defined
	a.accessKey = a.defaultAccessKey
	a.secretKey = a.defaultSecretKey
	// try to read from local metadata api
	if err := a.refreshInstanceRoleCredentials(); err != nil {
		return fmt.Errorf("cannot get instance creds: %w", err)
	}

	// read credentials from sts api, if role_arn is defined
	if a.roleARN != "" {
		if err := a.refreshRoleArnCredentials(); err != nil {
			return fmt.Errorf("failed to refresh Arn credentials: %w", err)
		}
	}
	if len(a.accessKey) == 0 {
		return fmt.Errorf("missing `access_key`, you can set it with AWS_ACCESS_KEY_ID env var, " +
			"directly at `ec2_sd_config` as `access_key` or use instance iam role")
	}
	if len(a.secretKey) == 0 {
		return fmt.Errorf("missing `secret_key`, you can set it with AWS_SECRET_ACCESS_KEY env var," +
			"directly at `ec2_sd_config` as `secret_key` or use instance iam role")
	}

	return nil
}

// refreshInstanceRoleCredentials makes request to local ec2 instance metadata service
// and tries to retrieve assigned iam role.
// updates apiConfig credentials
// https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
func (a *apiConfig) refreshInstanceRoleCredentials() error {
	instanceRoleName, err := getMetadataByPath("meta-data/iam/security-credentials/", a.client)
	if err != nil {
		switch e := err.Error(); {

		// instance doesnt have iam role assigned
		case strings.Contains(e, "got 404; want 200"):
			return nil

		// there is no metadata api
		// probably its not aws instance or its disabled
		case strings.Contains(e, "dial tcp 169.254.169.254:80: connect: no route to host"):
			return nil
		}
		return fmt.Errorf("cannot get instanceRoleName: %w", err)
	}

	credentials, err := getMetadataByPath("meta-data/iam/security-credentials/"+string(instanceRoleName), a.client)
	if err != nil {
		return fmt.Errorf("cannot get instanceCredentials with role: %w", err)
	}
	instanceCredentials := &InstanceRoleCredentialsResponse{}
	if err := json.Unmarshal(credentials, instanceCredentials); err != nil {
		return err
	}
	a.accessKey = instanceCredentials.AccessKeyId
	a.secretKey = instanceCredentials.SecretAccessKey
	a.token = instanceCredentials.Token
	t, err := time.Parse("2006-01-02T15:04:05Z", instanceCredentials.Expiration)
	if err != nil {
		return fmt.Errorf("cannot  parse instanceCredentials expiration time: %w", err)
	}
	a.tokenExpiration = t

	return nil
}

// IdentityDocument is identity document returned from AWS metadata server.
//
// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/instance-identity-documents.html
type IdentityDocument struct {
	Region string
}

// getMetadataByPath returns instance metadata by url path
func getMetadataByPath(apiPath string, client *http.Client) ([]byte, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html

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

// refreshRoleArnCredentials - retrieves credentials data from aws sts
// using role_arn
// updates apiConfig
func (a *apiConfig) refreshRoleArnCredentials() error {
	apiEndpoint := buildAPIEndpoint(a.region, a.endpoint, "sts")
	data, err := getStsAPIResponse(a, apiEndpoint)
	if err != nil {
		return err
	}
	stsCredentials := &StsCredentialsResponse{}
	if err := xml.Unmarshal(data, stsCredentials); err != nil {
		return fmt.Errorf("cannot unmarshal sts api response from xml: %w", err)
	}
	t, err := time.Parse("2006-01-02T15:04:05Z", stsCredentials.AssumeRoleResult.Credentials.Expiration)
	if err != nil {
		return fmt.Errorf("cannot parse sts response expiration time: %w", err)
	}
	// update variables and expiration time
	a.accessKey = stsCredentials.AssumeRoleResult.Credentials.AccessKeyId
	a.secretKey = stsCredentials.AssumeRoleResult.Credentials.SecretAccessKey
	a.token = stsCredentials.AssumeRoleResult.Credentials.SessionToken

	// update expiration time, if it is not set or less than response from metadata api
	if a.tokenExpiration.IsZero() || t.Unix() < a.tokenExpiration.Unix() {
		a.tokenExpiration = t
	}

	return nil

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

// getStsAPIResponse makes request to aws sts api with role_arn
// and returns temporary credentials with expiration time
// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
func getStsAPIResponse(cfg *apiConfig, endpoint string) ([]byte, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html
	apiURL := fmt.Sprintf("%s?Action=%s", endpoint, "AssumeRole")
	apiURL += "&Version=2011-06-15"
	apiURL += fmt.Sprintf("&RoleArn=%s", cfg.roleARN)
	// we have to provide unique session name for cloudtrail audit
	apiURL += "&RoleSessionName=vmagent-ec2-discovery"
	req, err := newSignedRequest(apiURL, "sts", cfg.region, cfg.accessKey, cfg.secretKey, cfg.token)
	if err != nil {
		return nil, fmt.Errorf("cannot create signed request: %w", err)
	}
	resp, err := cfg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot perform http request to %q: %w", apiURL, err)
	}
	return readResponseBody(resp, apiURL)

}

// getEC2APIResponse lists ec2 instances with pagination
func getEC2APIResponse(cfg *apiConfig, endpoint, action, nextPageToken string) ([]byte, error) {

	apiURL := fmt.Sprintf("%s?Action=%s", endpoint, url.QueryEscape(action))
	if len(cfg.filters) > 0 {
		apiURL += "&" + cfg.filters
	}
	if len(nextPageToken) > 0 {
		apiURL += fmt.Sprintf("&NextToken=%s", url.QueryEscape(nextPageToken))
	}
	apiURL += "&Version=2013-10-15"
	req, err := newSignedRequest(apiURL, "ec2", cfg.region, cfg.accessKey, cfg.secretKey, cfg.token)
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
