package azure

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs/fscore"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
)

var configMap = discoveryutil.NewConfigMap()

// Extract from the needed params from https://github.com/Azure/go-autorest/blob/7dd32b67be4e6c9386b9ba7b1c44a51263f05270/autorest/azure/environments.go#L61
type cloudEnvironmentEndpoints struct {
	ActiveDirectoryEndpoint string `json:"activeDirectoryEndpoint"`
	ResourceManagerEndpoint string `json:"resourceManagerEndpoint"`
}

// well-known azure cloud endpoints
// See https://github.com/Azure/go-autorest/blob/7dd32b67be4e6c9386b9ba7b1c44a51263f05270/autorest/azure/environments.go#L34
var cloudEnvironments = map[string]*cloudEnvironmentEndpoints{
	"AZURECHINACLOUD": {
		ActiveDirectoryEndpoint: "https://login.chinacloudapi.cn",
		ResourceManagerEndpoint: "https://management.chinacloudapi.cn",
	},
	"AZUREGERMANCLOUD": {
		ActiveDirectoryEndpoint: "https://login.microsoftonline.de",
		ResourceManagerEndpoint: "https://management.microsoftazure.de",
	},
	"AZURECLOUD": {
		ActiveDirectoryEndpoint: "https://login.microsoftonline.com",
		ResourceManagerEndpoint: "https://management.azure.com",
	},
	"AZUREPUBLICCLOUD": {
		ActiveDirectoryEndpoint: "https://login.microsoftonline.com",
		ResourceManagerEndpoint: "https://management.azure.com",
	},
	"AZUREUSGOVERNMENT": {
		ActiveDirectoryEndpoint: "https://login.microsoftonline.us",
		ResourceManagerEndpoint: "https://management.usgovcloudapi.net",
	},
	"AZUREUSGOVERNMENTCLOUD": {
		ActiveDirectoryEndpoint: "https://login.microsoftonline.us",
		ResourceManagerEndpoint: "https://management.usgovcloudapi.net",
	},
}

// apiConfig contains config for API server.
type apiConfig struct {
	c              *discoveryutil.Client
	port           int
	resourceGroup  string
	subscriptionID string
	tenantID       string

	refreshToken refreshTokenFunc
	// tokenLock guards auth token and tokenExpireDeadline
	tokenLock           sync.Mutex
	token               string
	tokenExpireDeadline time.Time

	// apiServerHost is only used for verifying the `nextLink` in response of the list API.
	apiServerHost string
}

type refreshTokenFunc func() (string, time.Duration, error)

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (any, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	if sdc.SubscriptionID == "" {
		return nil, fmt.Errorf("missing `subscription_id` config option")
	}
	port := sdc.Port
	if port == 0 {
		port = 80
	}

	ac, err := sdc.HTTPClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}

	environment := sdc.Environment
	if environment == "" {
		environment = "AZURECLOUD"
	}
	env, err := getCloudEnvByName(environment)
	if err != nil {
		return nil, fmt.Errorf("cannot read configs for `environment: %q`: %w", environment, err)
	}

	refreshToken, err := getRefreshTokenFunc(sdc, ac, proxyAC, env)
	if err != nil {
		return nil, err
	}
	c, err := discoveryutil.NewClient(env.ResourceManagerEndpoint, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot create client for %q: %w", env.ResourceManagerEndpoint, err)
	}
	// It's already verified in discoveryutil.NewClient so no need to check err.
	u, _ := url.Parse(c.APIServer())

	cfg := &apiConfig{
		c:              c,
		apiServerHost:  u.Hostname(),
		port:           port,
		resourceGroup:  sdc.ResourceGroup,
		subscriptionID: sdc.SubscriptionID,
		tenantID:       sdc.TenantID,

		refreshToken: refreshToken,
	}
	return cfg, nil
}

func getCloudEnvByName(name string) (*cloudEnvironmentEndpoints, error) {
	name = strings.ToUpper(name)
	// Special case, azure cloud k8s cluster, read content from file.
	// See https://github.com/Azure/go-autorest/blob/7dd32b67be4e6c9386b9ba7b1c44a51263f05270/autorest/azure/environments.go#L301
	if name == "AZURESTACKCLOUD" {
		return readCloudEndpointsFromFile(os.Getenv("AZURE_ENVIRONMENT_FILEPATH"))
	}
	env := cloudEnvironments[name]
	if env == nil {
		var supportedEnvs []string
		for envName := range cloudEnvironments {
			supportedEnvs = append(supportedEnvs, envName)
		}
		return nil, fmt.Errorf("unsupported `environment: %q`; supported values: %s", name, strings.Join(supportedEnvs, ","))
	}
	return env, nil
}

func readCloudEndpointsFromFile(filePath string) (*cloudEnvironmentEndpoints, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot file %q: %w", filePath, err)
	}
	var cee cloudEnvironmentEndpoints
	if err := json.Unmarshal(data, &cee); err != nil {
		return nil, fmt.Errorf("cannot parse cloud environment endpoints from file %q: %w", filePath, err)
	}
	return &cee, nil
}

func getRefreshTokenFunc(sdc *SDConfig, ac, proxyAC *promauth.Config, env *cloudEnvironmentEndpoints) (refreshTokenFunc, error) {
	var tokenEndpoint, tokenAPIPath string
	var modifyRequest func(request *http.Request)
	authenticationMethod := sdc.AuthenticationMethod
	if authenticationMethod == "" {
		authenticationMethod = "OAuth"
	}
	switch strings.ToLower(authenticationMethod) {
	case "oauth":
		if sdc.TenantID == "" {
			return nil, fmt.Errorf("missing `tenant_id` config option for `authentication_method: Oauth`")
		}
		if sdc.ClientID == "" {
			return nil, fmt.Errorf("missing `client_id` config option for `authentication_method: OAuth`")
		}
		if sdc.ClientSecret.String() == "" {
			return nil, fmt.Errorf("missing `client_secret` config option for `authentication_method: OAuth`")
		}
		q := url.Values{
			"grant_type":    []string{"client_credentials"},
			"client_id":     []string{sdc.ClientID},
			"client_secret": []string{sdc.ClientSecret.String()},
			"resource":      []string{env.ResourceManagerEndpoint},
		}
		authParams := q.Encode()
		tokenAPIPath = "/" + sdc.TenantID + "/oauth2/token"
		tokenEndpoint = env.ActiveDirectoryEndpoint
		modifyRequest = func(request *http.Request) {
			request.Body = io.NopCloser(strings.NewReader(authParams))
			request.Method = http.MethodPost
		}
	case "managedidentity":
		endpoint := "http://169.254.169.254/metadata/identity/oauth2/token"
		if ep := os.Getenv("MSI_ENDPOINT"); ep != "" {
			endpoint = ep
		}
		endpointURL, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("cannot parse MSI endpoint url %q: %w", endpoint, err)
		}
		q := endpointURL.Query()

		msiSecret := os.Getenv("MSI_SECRET")
		identityHeader := os.Getenv("IDENTITY_HEADER")
		clientIDParam := "client_id"
		apiVersion := "2018-02-01"
		if msiSecret != "" {
			clientIDParam = "clientid"
			apiVersion = "2017-09-01"
		}
		if identityHeader != "" {
			clientIDParam = "client_id"
			apiVersion = "2019-08-01"
		}
		q.Set("api-version", apiVersion)
		q.Set(clientIDParam, sdc.ClientID)
		q.Set("resource", env.ResourceManagerEndpoint)
		endpointURL.RawQuery = q.Encode()
		tokenAPIPath = endpointURL.RequestURI()
		tokenEndpoint = endpointURL.Scheme + "://" + endpointURL.Host
		modifyRequest = func(request *http.Request) {
			if msiSecret != "" {
				request.Header.Set("secret", msiSecret)
				if identityHeader != "" {
					request.Header.Set("X-IDENTITY-HEADER", msiSecret)
				}
			} else {
				request.Header.Set("Metadata", "true")
			}
		}
	case "workloadidentity":
		// See https://learn.microsoft.com/en-us/entra/identity-platform/v2-oauth2-client-creds-grant-flow#third-case-access-token-request-with-a-federated-credential
		tenantID := os.Getenv("AZURE_TENANT_ID")
		clientID := os.Getenv("AZURE_CLIENT_ID")
		tokenFile := os.Getenv("AZURE_FEDERATED_TOKEN_FILE")
		if tenantID == "" || clientID == "" || tokenFile == "" {
			return nil, fmt.Errorf("missing AZURE_TENANT_ID, AZURE_CLIENT_ID or AZURE_FEDERATED_TOKEN_FILE env var for `authentication_method: WorkloadIdentity`")
		}
		tokenAPIPath = "/" + tenantID + "/oauth2/v2.0/token"
		tokenEndpoint = env.ActiveDirectoryEndpoint
		modifyRequest = func(request *http.Request) {
			// The token file is rotated by Kubernetes, so it must be read before every token request.
			assertion, err := fscore.ReadPasswordFromFileOrHTTP(tokenFile)
			if err != nil {
				logger.Errorf("cannot read federated token from AZURE_FEDERATED_TOKEN_FILE: %s", err)
			}
			q := url.Values{
				"grant_type":            []string{"client_credentials"},
				"client_id":             []string{clientID},
				"client_assertion_type": []string{"urn:ietf:params:oauth:client-assertion-type:jwt-bearer"},
				"client_assertion":      []string{assertion},
				"scope":                 []string{env.ResourceManagerEndpoint + "/.default"},
			}
			request.Body = io.NopCloser(strings.NewReader(q.Encode()))
			request.Method = http.MethodPost
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	default:
		return nil, fmt.Errorf("unsupported `authentication_method: %q` only `OAuth`, `ManagedIdentity` and `WorkloadIdentity` are supported", authenticationMethod)
	}

	authClient, err := discoveryutil.NewClient(tokenEndpoint, ac, sdc.ProxyURL, proxyAC, &sdc.HTTPClientConfig)
	if err != nil {
		return nil, fmt.Errorf("cannot build auth client: %w", err)
	}
	refreshToken := func() (string, time.Duration, error) {
		data, err := authClient.GetAPIResponseWithReqParams(tokenAPIPath, modifyRequest)
		if err != nil {
			return "", 0, err
		}
		var tr tokenResponse
		if err := json.Unmarshal(data, &tr); err != nil {
			return "", 0, fmt.Errorf("cannot parse token auth response %q: %w", data, err)
		}

		expiresInSeconds, err := parseTokenExpiry(tr)
		if err != nil {
			return "", 0, err
		}
		return tr.AccessToken, time.Second * time.Duration(expiresInSeconds), nil
	}
	return refreshToken, nil
}

// parseTokenExpiry returns token expiry in seconds
func parseTokenExpiry(tr tokenResponse) (int64, error) {
	var expiresInSeconds int64
	var err error

	if tr.ExpiresIn == "" {
		var expiresOnSeconds int64
		expiresOnSeconds, err = strconv.ParseInt(tr.ExpiresOn, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse expiresOn=%q in auth token response: %w", tr.ExpiresOn, err)
		}
		expiresInSeconds = expiresOnSeconds - time.Now().Unix()
	} else {
		expiresInSeconds, err = tr.ExpiresIn.Int64()
		if err != nil {
			return 0, fmt.Errorf("cannot parse expiresIn=%q auth token response: %w", tr.ExpiresIn, err)
		}
	}

	return expiresInSeconds, nil
}

// mustGetAuthToken returns auth token
// in case of error, logs error and return empty token
func (ac *apiConfig) mustGetAuthToken() string {
	ac.tokenLock.Lock()
	defer ac.tokenLock.Unlock()

	ct := time.Now()
	if ac.tokenExpireDeadline.Sub(ct) > time.Second*30 {
		return ac.token
	}
	token, expiresDuration, err := ac.refreshToken()
	if err != nil {
		logger.Errorf("cannot refresh azure auth token: %s", err)
		return ""
	}
	ac.token = token
	ac.tokenExpireDeadline = ct.Add(expiresDuration)
	return ac.token
}

// tokenResponse represent response from oauth2 azure token service
//
// https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/how-to-use-vm-token#get-a-token-using-go
type tokenResponse struct {
	AccessToken string `json:"access_token"`
	// ExpiresIn is a string in v1 token responses and a number in v2.0 token responses.
	ExpiresIn json.Number `json:"expires_in"`
	ExpiresOn string      `json:"expires_on"`
}
