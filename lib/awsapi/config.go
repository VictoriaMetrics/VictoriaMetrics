package awsapi

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

// Config represent aws access configuration.
type Config struct {
	client  *http.Client
	region  string
	roleARN string

	// IRSA may use a different role for assume API call.
	// It can only be set via AWS_ROLE_ARN env variable.
	// See https://docs.aws.amazon.com/eks/latest/userguide/pod-configuration.html
	irsaRoleARN string

	webTokenPath       string
	containerTokenPath string

	ec2Endpoint string
	stsEndpoint string
	service     string

	// these keys are needed for obtaining creds.
	defaultAccessKey string
	defaultSecretKey string

	// Real credentials used for accessing EC2 API.
	creds     *credentials
	credsLock sync.Mutex
}

// credentials represent aws api credentials.
type credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	Token           string
	Expiration      time.Time
}

// NewConfig returns new AWS Config from the given args.
func NewConfig(ec2Endpoint, stsEndpoint, region, roleARN, accessKey, secretKey, service string) (*Config, error) {
	cfg := &Config{
		client:             http.DefaultClient,
		region:             region,
		roleARN:            roleARN,
		irsaRoleARN:        os.Getenv("AWS_ROLE_ARN"),
		containerTokenPath: os.Getenv("AWS_CONTAINER_AUTHORIZATION_TOKEN_FILE"),
		service:            service,
		defaultAccessKey:   os.Getenv("AWS_ACCESS_KEY_ID"),
		defaultSecretKey:   os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
	if cfg.service == "" {
		cfg.service = "aps"
	}
	if cfg.region == "" {
		r, err := getDefaultRegion(cfg.client)
		if err != nil {
			return nil, fmt.Errorf("cannot determine default AWS region: %w", err)
		}
		cfg.region = r
	}
	cfg.ec2Endpoint = buildAPIEndpoint(ec2Endpoint, cfg.region, "ec2")
	cfg.stsEndpoint = buildAPIEndpoint(stsEndpoint, cfg.region, "sts")
	cfg.webTokenPath = os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE")
	if cfg.webTokenPath != "" && cfg.irsaRoleARN == "" {
		return nil, fmt.Errorf("roleARN is missing for AWS_WEB_IDENTITY_TOKEN_FILE=%q; set it via env var AWS_ROLE_ARN", cfg.webTokenPath)
	}
	// explicitly set credentials has priority over env variables
	if len(accessKey) > 0 {
		cfg.defaultAccessKey = accessKey
	}
	if len(secretKey) > 0 {
		cfg.defaultSecretKey = secretKey
	}
	cfg.creds = &credentials{
		AccessKeyID:     cfg.defaultAccessKey,
		SecretAccessKey: cfg.defaultSecretKey,
	}

	return cfg, nil
}

// GetRegion returns region for the given cfg.
func (cfg *Config) GetRegion() string {
	return cfg.region
}

// GetEC2APIResponse performs EC2 API request with ghe given action.
//
// filtersQueryString must contain an optional percent-encoded query string for aws filters.
// This string can be obtained by calling GetFiltersQueryString().
// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html for examples.
// See also https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Filter.html
func (cfg *Config) GetEC2APIResponse(action, filtersQueryString, nextPageToken string) ([]byte, error) {
	ac, err := cfg.getFreshAPICredentials()
	if err != nil {
		return nil, err
	}
	apiURL := fmt.Sprintf("%s?Action=%s", cfg.ec2Endpoint, url.QueryEscape(action))
	if len(filtersQueryString) > 0 {
		apiURL += "&" + filtersQueryString
	}
	if len(nextPageToken) > 0 {
		apiURL += fmt.Sprintf("&NextToken=%s", url.QueryEscape(nextPageToken))
	}
	apiURL += "&Version=2016-11-15"
	req, err := newSignedGetRequest(apiURL, "ec2", cfg.region, ac)
	if err != nil {
		return nil, fmt.Errorf("cannot create signed request: %w", err)
	}
	resp, err := cfg.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot perform http request to %q: %w", apiURL, err)
	}
	return readResponseBody(resp, apiURL)
}

// SignRequest signs request for service access and payloadHash.
func (cfg *Config) SignRequest(req *http.Request, payloadHash string) error {
	ac, err := cfg.getFreshAPICredentials()
	if err != nil {
		return err
	}
	return signRequestWithTime(req, cfg.service, cfg.region, payloadHash, ac, time.Now().UTC())
}

func readResponseBody(resp *http.Response, apiURL string) ([]byte, error) {
	data, err := io.ReadAll(resp.Body)
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

func getDefaultRegion(client *http.Client) (string, error) {
	envRegion := os.Getenv("AWS_REGION")
	if envRegion != "" {
		return envRegion, nil
	}
	data, err := getMetadataByPath(client, "dynamic/instance-identity/document")
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
func (cfg *Config) getFreshAPICredentials() (*credentials, error) {
	cfg.credsLock.Lock()
	defer cfg.credsLock.Unlock()

	if len(cfg.defaultAccessKey) > 0 && len(cfg.defaultSecretKey) > 0 && len(cfg.roleARN) == 0 {
		// There is no need in refreshing statically set api credentials if roleARN isn't set.
		return cfg.creds, nil
	}
	if time.Until(cfg.creds.Expiration) > 10*time.Second {
		// credentials aren't expired yet.
		return cfg.creds, nil
	}
	// credentials have been expired. Update them.
	ac, err := cfg.getAPICredentials()
	if err != nil {
		return nil, fmt.Errorf("cannot obtain new EC2 API credentials: %w", err)
	}
	cfg.creds = ac
	return ac, nil
}

// getAPICredentials obtains new EC2 API credentials from instance metadata and role_arn.
func (cfg *Config) getAPICredentials() (*credentials, error) {
	acNew := &credentials{
		AccessKeyID:     cfg.defaultAccessKey,
		SecretAccessKey: cfg.defaultSecretKey,
	}
	fullURI := os.Getenv("AWS_CONTAINER_CREDENTIALS_FULL_URI")
	if relativeURI := os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"); len(relativeURI) > 0 {
		fullURI = "http://169.254.170.2" + relativeURI
	}
	switch {
	case len(acNew.AccessKeyID) > 0 && len(acNew.SecretAccessKey) > 0:
	case len(cfg.webTokenPath) > 0:
		token, err := os.ReadFile(cfg.webTokenPath)
		if err != nil {
			return nil, fmt.Errorf("cannot read webToken from path: %q, err: %w", cfg.webTokenPath, err)
		}
		return cfg.getRoleWebIdentityCredentials(string(token), cfg.irsaRoleARN)
	case len(fullURI) > 0:
		token := os.Getenv("AWS_CONTAINER_AUTHORIZATION_TOKEN")
		if len(token) == 0 && len(cfg.containerTokenPath) > 0 {
			t, err := os.ReadFile(cfg.containerTokenPath)
			if err != nil {
				return nil, fmt.Errorf("cannot read containerToken from path: %q, err: %w", cfg.containerTokenPath, err)
			}
			token = string(t)
		}
		ac, err := getCredentialsByPath(cfg.client, fullURI, token)
		if err != nil {
			return nil, err
		}
		acNew = ac
	default:
		// we need instance credentials if we do not have access keys
		ac, err := getInstanceRoleCredentials(cfg.client)
		if err != nil {
			return nil, fmt.Errorf("cannot obtain instance role credentials: %w", err)
		}
		acNew = ac
	}
	// read credentials from sts api, if role_arn is defined
	if len(cfg.roleARN) > 0 {
		ac, err := cfg.getRoleARNCredentials(acNew, cfg.roleARN)
		if err != nil {
			return nil, fmt.Errorf("cannot get credentials for role_arn %q: %w", cfg.roleARN, err)
		}
		acNew = ac
	}
	if len(acNew.AccessKeyID) == 0 {
		return nil, fmt.Errorf("missing AWS access_key; it may be set via env var AWS_ACCESS_KEY_ID or use instance iam role")
	}
	if len(acNew.SecretAccessKey) == 0 {
		return nil, fmt.Errorf("missing AWS secret_key; it may be set via env var AWS_SECRET_ACCESS_KEY or use instance iam role")
	}
	return acNew, nil
}

// getCredentialsByPath makes request to metadata service and retrieves container credentials
// https://docs.aws.amazon.com/sdkref/latest/guide/feature-container-credentials.html
func getCredentialsByPath(client *http.Client, uri, token string) (*credentials, error) {
	req, err := http.NewRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}
	if len(token) > 0 {
		req.Header.Add("Authorization", token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot get credentials from %s: %w", uri, err)
	}
	data, err := readResponseBody(resp, uri)
	if err != nil {
		return nil, err
	}
	return parseMetadataSecurityCredentials(data)
}

// getInstanceRoleCredentials makes request to local ec2 instance metadata service
// and tries to retrieve credentials from assigned iam role.
//
// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
func getInstanceRoleCredentials(client *http.Client) (*credentials, error) {
	instanceRoleName, err := getMetadataByPath(client, "meta-data/iam/security-credentials/")
	if err != nil {
		return nil, fmt.Errorf("cannot get instanceRoleName: %w", err)
	}
	data, err := getMetadataByPath(client, "meta-data/iam/security-credentials/"+string(instanceRoleName))
	if err != nil {
		return nil, fmt.Errorf("cannot get security credentials for instanceRoleName %q: %w", instanceRoleName, err)
	}
	return parseMetadataSecurityCredentials(data)
}

// parseMetadataSecurityCredentials parses apiCredentials from metadata response to http://169.254.169.254/latest/meta-data/iam/security-credentials/*
//
// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-roles-for-amazon-ec2.html
func parseMetadataSecurityCredentials(data []byte) (*credentials, error) {
	var msc MetadataSecurityCredentials
	if err := json.Unmarshal(data, &msc); err != nil {
		return nil, fmt.Errorf("cannot parse metadata security credentials from %q: %w", data, err)
	}
	return &credentials{
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
func getMetadataByPath(client *http.Client, apiPath string) ([]byte, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-instance-metadata-service.html

	// Obtain session token
	sessionTokenURL := "http://169.254.169.254/latest/api/token"
	req, err := http.NewRequest(http.MethodPut, sessionTokenURL, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request for IMDSv2 session token at url %q: %w", sessionTokenURL, err)
	}
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot obtain IMDSv2 session token from %q: %w", sessionTokenURL, err)
	}
	token, err := readResponseBody(resp, sessionTokenURL)
	if err != nil {
		return nil, fmt.Errorf("cannot read IMDSv2 session token from %q: %w", sessionTokenURL, err)
	}

	// Use session token in the request.
	apiURL := "http://169.254.169.254/latest/" + apiPath
	req, err = http.NewRequest(http.MethodGet, apiURL, nil)
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

// getRoleWebIdentityCredentials obtains credentials for the given roleARN with webToken.
//
// https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRoleWithWebIdentity.html
// aws IRSA for kubernetes.
// https://aws.amazon.com/blogs/opensource/introducing-fine-grained-iam-roles-service-accounts/
func (cfg *Config) getRoleWebIdentityCredentials(token, roleARN string) (*credentials, error) {
	data, err := cfg.getSTSAPIResponse("AssumeRoleWithWebIdentity", roleARN, func(apiURL string) (*http.Request, error) {
		apiURL += fmt.Sprintf("&WebIdentityToken=%s", url.QueryEscape(token))
		return http.NewRequest(http.MethodGet, apiURL, nil)
	})
	if err != nil {
		return nil, err
	}
	creds, err := parseARNCredentials(data, "AssumeRoleWithWebIdentity")
	if err != nil {
		return nil, err
	}
	if len(cfg.roleARN) > 0 {
		// need to assume a different role
		assumeCreds, err := cfg.getRoleARNCredentials(creds, cfg.roleARN)
		if err != nil {
			return nil, fmt.Errorf("cannot assume chained role=%q for roleARN=%q: %w", cfg.roleARN, roleARN, err)
		}
		if assumeCreds.Expiration.After(creds.Expiration) {
			assumeCreds.Expiration = creds.Expiration
		}
		return assumeCreds, nil
	}
	return creds, nil
}

// getSTSAPIResponse makes request to aws sts api with the given cfg and returns temporary credentials with expiration time.
//
// See https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
func (cfg *Config) getSTSAPIResponse(action string, roleARN string, reqBuilder func(apiURL string) (*http.Request, error)) ([]byte, error) {
	// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/Query-Requests.html
	apiURL := fmt.Sprintf("%s?Action=%s", cfg.stsEndpoint, action)
	apiURL += "&Version=2011-06-15"
	apiURL += fmt.Sprintf("&RoleArn=%s", roleARN)
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

// getRoleARNCredentials obtains credentials for the given roleARN.
func (cfg *Config) getRoleARNCredentials(creds *credentials, roleARN string) (*credentials, error) {
	data, err := cfg.getSTSAPIResponse("AssumeRole", roleARN, func(apiURL string) (*http.Request, error) {
		return newSignedGetRequest(apiURL, "sts", cfg.region, creds)
	})
	if err != nil {
		return nil, err
	}
	return parseARNCredentials(data, "AssumeRole")
}

// parseARNCredentials parses apiCredentials from AssumeRole response.
//
// See https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
func parseARNCredentials(data []byte, role string) (*credentials, error) {
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
	return &credentials{
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
		Credentials assumeCredentials `xml:"Credentials"`
	} `xml:"AssumeRoleResult"`
	AssumeRoleWithWebIdentityResult struct {
		Credentials assumeCredentials `xml:"Credentials"`
	} `xml:"AssumeRoleWithWebIdentityResult"`
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

// GetFiltersQueryString returns query string formed from the given filters.
func GetFiltersQueryString(filters []Filter) string {
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

// Filter is ec2 filter.
//
// See https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
// and https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_Filter.html
type Filter struct {
	Name   string   `yaml:"name"`
	Values []string `yaml:"values"`
}
