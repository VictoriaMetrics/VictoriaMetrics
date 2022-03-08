package remoteread

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
)

var (
	addr = flag.String("remoteRead.url", "", "Optional URL to VictoriaMetrics or vmselect that will be used to restore alerts "+
		"state. This configuration makes sense only if `vmalert` was configured with `remoteWrite.url` before and has been successfully persisted its state. "+
		"E.g. http://127.0.0.1:8428. See also -remoteRead.disablePathAppend")

	basicAuthUsername     = flag.String("remoteRead.basicAuth.username", "", "Optional basic auth username for -remoteRead.url")
	basicAuthPassword     = flag.String("remoteRead.basicAuth.password", "", "Optional basic auth password for -remoteRead.url")
	basicAuthPasswordFile = flag.String("remoteRead.basicAuth.passwordFile", "", "Optional path to basic auth password to use for -remoteRead.url")

	bearerToken     = flag.String("remoteRead.bearerToken", "", "Optional bearer auth token to use for -remoteRead.url.")
	bearerTokenFile = flag.String("remoteRead.bearerTokenFile", "", "Optional path to bearer token file to use for -remoteRead.url.")

	authorizationType            = flag.String("remoteRead.type", "", "Optional authorization type (`basic or bearer`) for -remoteRead.url")
	authorizationCredentials     = flag.String("remoteRead.credentials", "", "Optional authorization credentials for -remoteRead.url")
	authorizationCredentialsFile = flag.String("remoteRead.credentialsFile", "", "Optional path to authorization credentials file for -remoteRead.url")

	tlsInsecureSkipVerify = flag.Bool("remoteRead.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -remoteRead.url")
	tlsCertFile           = flag.String("remoteRead.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -remoteRead.url")
	tlsKeyFile            = flag.String("remoteRead.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -remoteRead.url")
	tlsCAFile             = flag.String("remoteRead.tlsCAFile", "", "Optional path to TLS CA file to use for verifying connections to -remoteRead.url. "+
		"By default system CA is used")
	tlsServerName = flag.String("remoteRead.tlsServerName", "", "Optional TLS server name to use for connections to -remoteRead.url. "+
		"By default the server name from -remoteRead.url is used")

	oauth2ClientID = flag.String("remoteRead.oauth2.clientID", "", "Optional OAuth2 clientID to use for -remoteRead.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteRead.url")
	oauth2ClientSecret = flag.String("remoteRead.oauth2.clientSecret", "", "Optional OAuth2 clientSecret to use for -remoteRead.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteRead.url")
	oauth2ClientSecretFile = flag.String("remoteRead.oauth2.clientSecretFile", "", "Optional OAuth2 clientSecretFile to use for -remoteRead.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteRead.url")
	oauth2TokenURL = flag.String("remoteRead.oauth2.tokenUrl", "", "Optional OAuth2 tokenURL to use for -remoteRead.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteRead.url")
	oauth2Scopes = flag.String("remoteRead.oauth2.scopes", "", "Optional OAuth2 scopes to use for -remoteRead.url. Scopes must be delimited by ';'. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteRead.url")

	disablePathAppend = flag.Bool("remoteRead.disablePathAppend", false, "Whether to disable automatic appending of '/api/v1/query' path to the configured -remoteRead.url.")
)

// Init creates a Querier from provided flag values.
// Returns nil if addr flag wasn't set.
func Init() (datasource.QuerierBuilder, error) {
	if *addr == "" {
		return nil, nil
	}
	tr, err := utils.Transport(*addr, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	authCfg, err := utils.AuthConfig(
		utils.WithAuthorization(*authorizationType, *authorizationCredentials, *authorizationCredentialsFile),
		utils.WithBasicAuth(*basicAuthUsername, *basicAuthPassword, *basicAuthPasswordFile),
		utils.WithBearer(*bearerToken, *bearerTokenFile),
		utils.WithOAuth(*oauth2ClientID, *oauth2ClientSecret, *oauth2ClientSecretFile, *oauth2TokenURL, *oauth2Scopes))
	if err != nil {
		return nil, fmt.Errorf("failed to configure auth: %w", err)
	}
	c := &http.Client{Transport: tr}
	return datasource.NewVMStorage(*addr, authCfg, 0, 0, false, c, *disablePathAppend), nil
}
