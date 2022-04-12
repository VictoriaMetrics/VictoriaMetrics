package aws

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

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type credentials struct {
	SecretAccessKey string
	AccessKeyID     string
	Token           string
	Expiration      time.Time
}

// Config represent aws access configuration.
type Config struct {
	client       *http.Client
	region       string
	roleARN      string
	webTokenPath string
	filters      string

	ec2Endpoint string
	stsEndpoint string

	// these keys are needed for obtaining creds.
	defaultAccessKey string
	defaultSecretKey string

	// Real credentials used for accessing EC2 API.
	creds     *credentials
	credsLock sync.Mutex
}

// NewConfig returns new AWS Config.
func NewConfig(region, roleARN, accessKey string, secretKey *promauth.Secret) (*Config, error) {
	var err error
	cfg := Config{
		client:           http.DefaultClient,
		region:           region,
		roleARN:          roleARN,
		defaultAccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
		defaultSecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
	cfg.region = region

	if cfg.region == "" {
		cfg.region, err = getDefaultRegion(cfg.client)
		if err != nil {
			return nil, err
		}
	}
	cfg.stsEndpoint = buildAPIEndpoint(cfg.stsEndpoint, cfg.region, "sts")
	cfg.ec2Endpoint = buildAPIEndpoint(cfg.ec2Endpoint, cfg.region, "ec2")

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
	if len(accessKey) > 0 {
		cfg.defaultAccessKey = accessKey
	}
	if secretKey != nil && len(secretKey.String()) > 0 {
		cfg.defaultSecretKey = secretKey.String()
	}
	cfg.creds = &credentials{
		AccessKeyID:     cfg.defaultAccessKey,
		SecretAccessKey: cfg.defaultSecretKey,
	}
	return &cfg, nil
}

// SetFilters adds filters to given config.
func (cfg *Config) SetFilters(filters string) {
	cfg.filters = filters
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
		// There is no need in refreshing statically set api credentials if `role_arn` isn't set.
		return cfg.creds, nil
	}
	if time.Until(cfg.creds.Expiration) > 10*time.Second {
		// credentials aren't expired yet.
		return cfg.creds, nil
	}
	// credentials have been expired. Update them.
	ac, err := getAPICredentials(cfg)
	if err != nil {
		return nil, err
	}
	cfg.creds = ac
	return ac, nil
}

// getAPICredentials obtains new EC2 API credentials from instance metadata and role_arn.
func getAPICredentials(cfg *Config) (*credentials, error) {
	acNew := &credentials{
		AccessKeyID:     cfg.defaultAccessKey,
		SecretAccessKey: cfg.defaultSecretKey,
	}
	if len(cfg.webTokenPath) > 0 {
		token, err := ioutil.ReadFile(cfg.webTokenPath)
		if err != nil {
			return nil, fmt.Errorf("cannot read webToken from path: %q, err: %w", cfg.webTokenPath, err)
		}
		return getRoleWebIdentityCredentials(cfg, string(token))
	}

	if ecsMetaURI := os.Getenv("AWS_CONTAINER_CREDENTIALS_RELATIVE_URI"); len(ecsMetaURI) > 0 {
		path := "http://169.254.170.2" + ecsMetaURI
		return getECSRoleCredentialsByPath(cfg.client, path)
	}

	// we need instance credentials if dont have access keys
	if len(acNew.AccessKeyID) == 0 && len(acNew.SecretAccessKey) == 0 {
		ac, err := getInstanceRoleCredentials(cfg.client)
		if err != nil {
			return nil, err
		}
		acNew = ac
	}

	// read credentials from sts api, if role_arn is defined
	if len(cfg.roleARN) > 0 {
		ac, err := getRoleARNCredentials(cfg, acNew)
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
func getECSRoleCredentialsByPath(client *http.Client, path string) (*credentials, error) {
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
func getInstanceRoleCredentials(client *http.Client) (*credentials, error) {
	instanceRoleName, err := getMetadataByPath(client, "meta-data/iam/security-credentials/")
	if err != nil {
		return nil, fmt.Errorf("cannot get instanceRoleName: %w", err)
	}
	data, err := getMetadataByPath(client, "meta-data/iam/security-credentials/"+string(instanceRoleName))
	if err != nil {
		return nil, fmt.Errorf("cannot get security credentails for instanceRoleName %q: %w", instanceRoleName, err)
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
func getRoleWebIdentityCredentials(cfg *Config, token string) (*credentials, error) {
	data, err := getSTSAPIResponse("AssumeRoleWithWebIdentity", cfg, func(apiURL string) (*http.Request, error) {
		apiURL += fmt.Sprintf("&WebIdentityToken=%s", url.QueryEscape(token))
		return http.NewRequest("GET", apiURL, nil)
	})
	if err != nil {
		return nil, err
	}
	return parseARNCredentials(data, "AssumeRoleWithWebIdentity")
}

// getRoleARNCredentials obtains credentials fo the given roleARN.
func getRoleARNCredentials(cfg *Config, creds *credentials) (*credentials, error) {
	data, err := getSTSAPIResponse("AssumeRole", cfg, func(apiURL string) (*http.Request, error) {
		return NewSignedGetRequestWithTime(apiURL, "sts", cfg.region, creds, time.Now().UTC())
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
	var arr assumeRoleResponse
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

// assumeRoleResponse represents AssumeRole response
//
// See https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html
type assumeRoleResponse struct {
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
