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
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

type apiConfig struct {
	region       string
	roleARN      string
	webTokenPath string
	filters      string
	port         int

	ec2Endpoint string
	stsEndpoint string

	// these keys are needed for obtaining creds.
	defaultAccessKey string
	defaultSecretKey string

	// Real credentials used for accessing EC2 API.
	creds     *apiCredentials
	credsLock sync.Mutex

	// A map from AZ name to AZ id.
	azMap     map[string]string
	azMapLock sync.Mutex
}

// apiCredentials represents aws api credentials
type apiCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	Token           string
	Expiration      time.Time
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
	filters := getFiltersQueryString(sdc.Filters)
	port := 80
	if sdc.Port != nil {
		port = *sdc.Port
	}
	cfg := &apiConfig{
		region:  region,
		roleARN: sdc.RoleARN,
		filters: filters,
		port:    port,
	}
	cfg.ec2Endpoint = buildAPIEndpoint(sdc.Endpoint, region, "ec2")
	cfg.stsEndpoint = buildAPIEndpoint(sdc.Endpoint, region, "sts")

	if cfg.roleARN == "" {
		cfg.roleARN = os.Getenv("AWS_ROLE_ARN")
	}
	cfg.webTokenPath = os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	if cfg.webTokenPath != "" && cfg.roleARN == "" {
		return nil, fmt.Errorf("roleARN is missing for AWS_WEB_IDENTITY_TOKEN_FILE=%q, set it either in `ec2_sd_config` or via env var AWS_ROLE_ARN", cfg.webTokenPath)
	}
	// explicitly set credentials has priority over env variables
	cfg.defaultAccessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	cfg.defaultSecretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	if len(sdc.AccessKey) > 0 {
		cfg.defaultAccessKey = sdc.AccessKey
	}
	if sdc.SecretKey != nil {
		cfg.defaultSecretKey = sdc.SecretKey.String()
	}
	cfg.creds = &apiCredentials{
		AccessKeyID:     cfg.defaultAccessKey,
		SecretAccessKey: cfg.defaultSecretKey,
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
	envRegion := os.Getenv("AWS_REGION")
	if envRegion != "" {
		return envRegion, nil
	}
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

// getFreshAPICredentials returns fresh EC2 API credentials.
//
// The credentials are refreshed if needed.
func (cfg *apiConfig) getFreshAPICredentials() (*apiCredentials, error) {
	cfg.credsLock.Lock()
	defer cfg.credsLock.Unlock()

	if len(cfg.defaultAccessKey) > 0 && len(cfg.defaultSecretKey) > 0 && len(cfg.roleARN) == 0 {
		// There is no need in refreshing statically set api credentials if `role_arn` isn't set.
		return cfg.creds, nil
	}
	if time.Until(cfg.creds.Expiration) > 10*time.Second {
		// Credentials aren't expired yet.
		return cfg.creds, nil
	}
	// Credentials have been expired. Update them.
	ac, err := getAPICredentials(cfg)
	if err != nil {
		return nil, err
	}
	cfg.creds = ac
	return ac, nil
}

// getAPICredentials obtains new EC2 API credentials from instance metadata and role_arn.
func getAPICredentials(cfg *apiConfig) (*apiCredentials, error) {
	acNew := &apiCredentials{
		AccessKeyID:     cfg.defaultAccessKey,
		SecretAccessKey: cfg.defaultSecretKey,
	}
	if len(cfg.webTokenPath) > 0 {
		token, err := ioutil.ReadFile(cfg.webTokenPath)
		if err != nil {
			return nil, fmt.Errorf("cannot read webToken from path: %q, err: %w", cfg.webTokenPath, err)
		}
		return getRoleWebIdentityCredentials(cfg.stsEndpoint, cfg.roleARN, string(token))
	}

	if ecsMetaURI := os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"); len(ecsMetaURI) > 0 {
		path := "http://169.254.170.2" + ecsMetaURI
		return getECSRoleCredentialsByPath(path)
	}

	// we need instance credentials if dont have access keys
	if len(acNew.AccessKeyID) == 0 && len(acNew.SecretAccessKey) == 0 {
		ac, err := getInstanceRoleCredentials()
		if err != nil {
			return nil, err
		}
		acNew = ac
	}

	// read credentials from sts api, if role_arn is defined
	if len(cfg.roleARN) > 0 {
		ac, err := getRoleARNCredentials(cfg.region, cfg.stsEndpoint, cfg.roleARN, acNew)
		if err != nil {
			return nil, fmt.Errorf("cannot get credentials for role_arn %q: %w", cfg.roleARN, err)
		}
		acNew = ac
	}
	if len(acNew.AccessKeyID) == 0 {
		return nil, fmt.Errorf("missing `access_key`, you can set it with env var AWS_ACCESS_KEY_ID, " +
			"directly at `ec2_sd_config` as `access_key` or use instance iam role")
	}
	if len(acNew.SecretAccessKey) == 0 {
		return nil, fmt.Errorf("missing `secret_key`, you can set it with env var AWS_SECRET_ACCESS_KEY," +
			"directly at `ec2_sd_config` as `secret_key` or use instance iam role")
	}
	return acNew, nil
}

// getECSRoleCredentialsByPath makes request to ecs metadata service
// and retrieves instances credentails
// https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-iam-roles.html
func getECSRoleCredentialsByPath(path string) (*apiCredentials, error) {
	client := discoveryutils.GetHTTPClient()
	resp, err := client.Get(path)
	if err != nil {
		return nil, fmt.Errorf("cannot get ECS instance role credentials: %w", err)
	}
	data, err := readResponseBody(resp, path)
	if err != nil {
		return nil, err
	}
	return parseMetadataSecurityCredentials(data)
}

// getInstanceRoleCredentials makes request to local ec2 instance metadata service
// and tries to retrieve credentials from assigned iam role.
//
// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
func getInstanceRoleCredentials() (*apiCredentials, error) {
	instanceRoleName, err := getMetadataByPath("meta-data/iam/security-credentials/")
	if err != nil {
		return nil, fmt.Errorf("cannot get instanceRoleName: %w", err)
	}
	data, err := getMetadataByPath("meta-data/iam/security-credentials/" + string(instanceRoleName))
	if err != nil {
		return nil, fmt.Errorf("cannot get security credentails for instanceRoleName %q: %w", instanceRoleName, err)
	}
	return parseMetadataSecurityCredentials(data)
}

// parseMetadataSecurityCredentials parses apiCredentials from metadata response to http://169.254.169.254/latest/meta-data/iam/security-credentials/*
//
// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
func parseMetadataSecurityCredentials(data []byte) (*apiCredentials, error) {
	var msc MetadataSecurityCredentials
	if err := json.Unmarshal(data, &msc); err != nil {
		return nil, fmt.Errorf("cannot parse metadata security credentials from %q: %w", data, err)
	}
	return &apiCredentials{
		AccessKeyID:     msc.AccessKeyID,
		SecretAccessKey: msc.SecretAccessKey,
		Token:           msc.Token,
		Expiration:      msc.Expiration,
	}, nil
}

// MetadataSecurityCredentials represents credentials obtained from http://169.254.169.254/latest/meta-data/iam/security-credentials/*
//
// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
type MetadataSecurityCredentials struct {
	AccessKeyID     string    `json:"AccessKeyId"`
	SecretAccessKey string    `json:"SecretAccessKey"`
	Token           string    `json:"Token"`
	Expiration      time.Time `json:"Expiration"`
}

// getMetadataByPath returns instance metadata by url path
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

// getRoleWebIdentityCredentials obtains credentials fo the given roleARN with webToken.
// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html
// aws IRSA for kubernetes.
// https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
func getRoleWebIdentityCredentials(stsEndpoint, roleARN string, token string) (*apiCredentials, error) {
	data, err := getSTSAPIResponse("AssumeRoleWithWebIdentity", stsEndpoint, roleARN, func(apiURL string) (*http.Request, error) {
		apiURL += fmt.Sprintf("&WebIdentityToken=%s", url.QueryEscape(token))
		return http.NewRequest("GET", apiURL, nil)
	})
	if err != nil {
		return nil, err
	}
	return parseARNCredentials(data, "AssumeRoleWithWebIdentity")
}

// getRoleARNCredentials obtains credentials fo the given roleARN.
func getRoleARNCredentials(region, stsEndpoint, roleARN string, creds *apiCredentials) (*apiCredentials, error) {
	data, err := getSTSAPIResponse("AssumeRole", stsEndpoint, roleARN, func(apiURL string) (*http.Request, error) {
		return newSignedRequest(apiURL, "sts", region, creds)
	})
	if err != nil {
		return nil, err
	}
	return parseARNCredentials(data, "AssumeRole")
}

// parseARNCredentials parses apiCredentials from AssumeRole response.
//
// See https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
func parseARNCredentials(data []byte, role string) (*apiCredentials, error) {
	var arr AssumeRoleResponse
	if err := xml.Unmarshal(data, &arr); err != nil {
		return nil, fmt.Errorf("cannot parse AssumeRoleResponse response from %q: %w", data, err)
	}
	var cred assumeCredentials
	switch role {
	case "AssumeRole":
		cred = arr.AssumeRoleResult.Credentials
	case "AssumeRoleWithWebIdentity":
		cred = arr.AssumeRoleWithWebIdentityResult.Credentials
	default:
		logger.Panicf("BUG: unexpected role: %q", role)
	}
	return &apiCredentials{
		AccessKeyID:     cred.AccessKeyID,
		SecretAccessKey: cred.SecretAccessKey,
		Token:           cred.SessionToken,
		Expiration:      cred.Expiration,
	}, nil
}

type assumeCredentials struct {
	AccessKeyID     string    `xml:"AccessKeyId"`
	SecretAccessKey string    `xml:"SecretAccessKey"`
	SessionToken    string    `xml:"SessionToken"`
	Expiration      time.Time `xml:"Expiration"`
}

// AssumeRoleResponse represents AssumeRole response
//
// See https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
type AssumeRoleResponse struct {
	AssumeRoleResult struct {
		Credentials assumeCredentials
	}
	AssumeRoleWithWebIdentityResult struct {
		Credentials assumeCredentials
	}
}

// buildAPIEndpoint creates endpoint for aws api access
func buildAPIEndpoint(customEndpoint, region, service string) string {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html
	if len(customEndpoint) == 0 {
		return fmt.Sprintf("https://%s.%s.amazonaws.com/", service, region)
	}
	endpoint := customEndpoint
	// endpoint may contain only hostname. Convert it to proper url then.
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	if !strings.HasSuffix(endpoint, "/") {
		endpoint += "/"
	}
	return endpoint
}

// getSTSAPIResponse makes request to aws sts api with roleARN
// and returns temporary credentials with expiration time
//
// See https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
func getSTSAPIResponse(action, stsEndpoint, roleARN string, reqBuilder func(apiURL string) (*http.Request, error)) ([]byte, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html
	apiURL := fmt.Sprintf("%s?Action=%s", stsEndpoint, action)
	apiURL += "&Version=2011-06-15"
	apiURL += fmt.Sprintf("&RoleArn=%s", roleARN)
	// we have to provide unique session name for cloudtrail audit
	apiURL += "&RoleSessionName=vmagent-ec2-discovery"
	req, err := reqBuilder(apiURL)
	if err != nil {
		return nil, fmt.Errorf("cannot create signed request: %w", err)
	}
	resp, err := discoveryutils.GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot perform http request to %q: %w", apiURL, err)
	}
	return readResponseBody(resp, apiURL)

}

// getEC2APIResponse performs EC2 API request with given action.
func getEC2APIResponse(cfg *apiConfig, action, filters, nextPageToken string) ([]byte, error) {
	ac, err := cfg.getFreshAPICredentials()
	if err != nil {
		return nil, fmt.Errorf("cannot obtain fresh credentials for EC2 API: %w", err)
	}
	apiURL := fmt.Sprintf("%s?Action=%s", cfg.ec2Endpoint, url.QueryEscape(action))
	if len(filters) > 0 {
		apiURL += "&" + filters
	}
	if len(nextPageToken) > 0 {
		apiURL += fmt.Sprintf("&NextToken=%s", url.QueryEscape(nextPageToken))
	}
	apiURL += "&Version=2013-10-15"
	req, err := newSignedRequest(apiURL, "ec2", cfg.region, ac)
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
