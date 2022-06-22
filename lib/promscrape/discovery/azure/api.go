package azure

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/fasthttp"
)

var configMap = discoveryutils.NewConfigMap()

type cloudEnvironmentEndpoints struct {
	ActiveDirectoryEndpoint string `json:"activeDirectoryEndpoint"`
	ResourceManagerEndpoint string `json:"resourceManagerEndpoint"`
}

// well-known azure cloud endpoints
var cloudEnvironments = map[string]*cloudEnvironmentEndpoints{
	"AZURECHINACLOUD":        &chinaCloud,
	"AZUREGERMANCLOUD":       &germanCloud,
	"AZURECLOUD":             &publicCloud,
	"AZUREPUBLICCLOUD":       &publicCloud,
	"AZUREUSGOVERNMENT":      &usGovernmentCloud,
	"AZUREUSGOVERNMENTCLOUD": &usGovernmentCloud,
}

var (
	chinaCloud = cloudEnvironmentEndpoints{
		ActiveDirectoryEndpoint: "https://login.chinacloudapi.cn",
		ResourceManagerEndpoint: "https://management.chinacloudapi.cn",
	}
	germanCloud = cloudEnvironmentEndpoints{
		ActiveDirectoryEndpoint: "https://login.microsoftonline.de",
		ResourceManagerEndpoint: "https://management.microsoftazure.de",
	}
	publicCloud = cloudEnvironmentEndpoints{
		ActiveDirectoryEndpoint: "https://login.microsoftonline.com",
		ResourceManagerEndpoint: "https://management.azure.com",
	}
	usGovernmentCloud = cloudEnvironmentEndpoints{
		ActiveDirectoryEndpoint: "https://login.microsoftonline.us",
		ResourceManagerEndpoint: "https://management.usgovcloudapi.net",
	}
)

// apiConfig contains config for API server.
type apiConfig struct {
	c              *discoveryutils.Client
	port           int
	resourceGroup  string
	subscriptionID string
	tenantID       string

	refreshToken tokenRefresher
	// guards auth token and expiration
	tokenLock  sync.Mutex
	authToken  string
	expiration time.Time
}

type tokenRefresher func() (string, time.Duration, error)

func getAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc, baseDir) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig, baseDir string) (*apiConfig, error) {
	hcc := sdc.HTTPClientConfig

	ac, err := hcc.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse auth config: %w", err)
	}
	proxyAC, err := sdc.ProxyClientConfig.NewConfig(baseDir)
	if err != nil {
		return nil, fmt.Errorf("cannot parse proxy auth config: %w", err)
	}

	cloudEndpoints, err := getCloudEnvByName(sdc.Environment)
	if err != nil {
		return nil, err
	}

	tr, err := newTokenRefresher(sdc, ac, proxyAC, cloudEndpoints)
	if err != nil {
		return nil, err
	}

	client, err := discoveryutils.NewClient(cloudEndpoints.ResourceManagerEndpoint, ac, sdc.ProxyURL, proxyAC)
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP client for %q: %w", cloudEndpoints.ResourceManagerEndpoint, err)
	}
	cfg := apiConfig{
		c:              client,
		subscriptionID: sdc.SubscriptionID,
		refreshToken:   tr,
		resourceGroup:  sdc.ResourceGroup,
		port:           sdc.Port,
		tenantID:       sdc.TenantID,
	}

	return &cfg, nil
}

func getCloudEnvByName(name string) (*cloudEnvironmentEndpoints, error) {

	name = strings.ToUpper(name)
	// special case, azure cloud k8s cluster, read content from file
	if name == "AZURESTACKCLOUD" {
		return readCloudEndpointsFromFile(os.Getenv("AZURE_ENVIRONMENT_FILEPATH"))
	}
	env := cloudEnvironments[name]
	if env == nil {
		var supportedEnvs []string
		for envName := range cloudEnvironments {
			supportedEnvs = append(supportedEnvs, envName)
		}

		return nil, fmt.Errorf("incorrect value for azure `environment` param: %q, supported values: %s", name, strings.Join(supportedEnvs, ","))
	}
	return env, nil
}

func readCloudEndpointsFromFile(filePath string) (*cloudEnvironmentEndpoints, error) {
	fileContent, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read cloud env endpoints from file %q:  %w", filePath, err)
	}
	var cee cloudEnvironmentEndpoints
	if err := json.Unmarshal(fileContent, &cee); err != nil {
		return nil, fmt.Errorf("cannot parse cloud env endpoints from file %q: %w", filePath, err)
	}
	return &cee, nil
}

func newTokenRefresher(sdc *SDConfig, ac, proxyAc *promauth.Config, cloudEndpoints *cloudEnvironmentEndpoints) (tokenRefresher, error) {

	var tokenEndpoint, tokenAPIPath string
	var modifyRequest func(request *fasthttp.Request)
	switch sdc.AuthenticationMethod {
	case "OAuth":
		q := make(url.Values)
		q.Set("grant_type", "client_credentials")
		q.Set("client_id", sdc.ClientID)
		q.Set("client_secret", sdc.ClientSecret.String())
		q.Set("resource", cloudEndpoints.ResourceManagerEndpoint)
		authParams := q.Encode()
		tokenAPIPath = "/" + sdc.TenantID + "/oauth2/token" // + authParams
		tokenEndpoint = cloudEndpoints.ActiveDirectoryEndpoint
		modifyRequest = func(request *fasthttp.Request) {
			request.SetBody([]byte(authParams))
			request.Header.SetMethod("POST")
		}
	case "ManagedIdentity":
		endpoint := resolveMSIEndpoint()
		endpointURL, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("cannot parse MSI endpoint url %q: %w", endpoint, err)
		}
		q := endpointURL.Query()

		msiSecret := os.Getenv("MSI_SECRET")
		clientIDParam := "client_id"
		apiVersion := "2018-02-01"
		isAppService := len(msiSecret) > 0
		if isAppService {
			clientIDParam = "clientid"
			apiVersion = "2017-09-01"
		}
		q.Set("api-version", apiVersion)
		q.Set(clientIDParam, sdc.ClientID)
		q.Set("resource", cloudEndpoints.ResourceManagerEndpoint)
		endpointURL.RawQuery = q.Encode()
		tokenAPIPath = endpointURL.RequestURI()

		tokenEndpoint = endpointURL.Scheme + "://" + endpointURL.Host
		modifyRequest = func(request *fasthttp.Request) {
			if !isAppService {
				request.Header.Set("Metadata", "true")
			}
			if len(msiSecret) > 0 {
				request.Header.Set("secret", msiSecret)
			}
		}
	default:
		logger.Fatalf("BUG, unexpected value: %s", sdc.AuthenticationMethod)
	}

	authClient, err := discoveryutils.NewClient(tokenEndpoint, ac, sdc.ProxyURL, proxyAc)
	if err != nil {
		return nil, fmt.Errorf("cannot build authorization client: %w", err)
	}
	return func() (string, time.Duration, error) {
		data, err := authClient.GetAPIResponseWithReqParams(tokenAPIPath, modifyRequest)
		if err != nil {
			return "", 0, err
		}
		var tr tokenResponse
		if err := json.Unmarshal(data, &tr); err != nil {
			return "", 0, fmt.Errorf("cannot parse token auth response: %q : %w", string(data), err)
		}
		expiresInSeconds, err := strconv.ParseInt(tr.ExpiresIn, 10, 64)
		if err != nil {
			return "", 0, fmt.Errorf("cannot parse auth token expiresIn param: expects int64 value, got: %s", tr.ExpiresIn)
		}
		return tr.AccessToken, time.Second * time.Duration(expiresInSeconds), nil
	}, nil
}

// mustGetAuthToken returns auth token
// in case of error, logs error and return empty token
func (ac *apiConfig) mustGetAuthToken() string {
	ac.tokenLock.Lock()
	defer ac.tokenLock.Unlock()
	if time.Until(ac.expiration) > time.Second*30 {
		return ac.authToken
	}
	ct := time.Now()
	token, expiresDuration, err := ac.refreshToken()
	if err != nil {
		logger.Errorf("cannot refresh azure auth token: %s", err)
		return ""
	}
	ac.authToken = token
	ac.expiration = ct.Add(expiresDuration)
	return ac.authToken
}

// tokenResponse represent response from oauth2 azure token service
// https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/how-to-use-vm-token#get-a-token-using-go
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    string `json:"expires_in"`
	ExpiresOn    string `json:"expires_on"`
	NotBefore    string `json:"not_before"`
	Resource     string `json:"resource"`
	TokenType    string `json:"token_type"`
}

// resolveMSIEndpoint returns endpoint for token request
// possible endpoints:
// MSI - for standard Virtual machines
// AppService - for lambda function apps
// CloudShell - not supported, for web browsers
func resolveMSIEndpoint() string {
	msiEndpoint := os.Getenv("MSI_ENDPOINT")
	if len(msiEndpoint) > 0 {
		return msiEndpoint
	}
	return "http://169.254.169.254/metadata/identity/oauth2/token"
}
