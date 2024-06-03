package remoteread

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
)

var (
	addr = flag.String("remoteRead.url", "", "Optional URL to datasource compatible with Prometheus HTTP API. It can be single node VictoriaMetrics or vmselect."+
		"Remote read is used to restore alerts state."+
		"This configuration makes sense only if `vmalert` was configured with `remoteWrite.url` before and has been successfully persisted its state. "+
		"Supports address in the form of IP address with a port (e.g., 127.0.0.1:8428) or DNS SRV record. "+
		"See also '-remoteRead.disablePathAppend', '-remoteRead.showURL'.")

	showRemoteReadURL = flag.Bool("remoteRead.showURL", false, "Whether to show -remoteRead.url in the exported metrics. "+
		"It is hidden by default, since it can contain sensitive info such as auth key")

	headers = flag.String("remoteRead.headers", "", "Optional HTTP headers to send with each request to the corresponding -remoteRead.url. "+
		"For example, -remoteRead.headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding -remoteRead.url. "+
		"Multiple headers must be delimited by '^^': -remoteRead.headers='header1:value1^^header2:value2'")

	basicAuthUsername     = flag.String("remoteRead.basicAuth.username", "", "Optional basic auth username for -remoteRead.url")
	basicAuthPassword     = flag.String("remoteRead.basicAuth.password", "", "Optional basic auth password for -remoteRead.url")
	basicAuthPasswordFile = flag.String("remoteRead.basicAuth.passwordFile", "", "Optional path to basic auth password to use for -remoteRead.url")

	bearerToken     = flag.String("remoteRead.bearerToken", "", "Optional bearer auth token to use for -remoteRead.url.")
	bearerTokenFile = flag.String("remoteRead.bearerTokenFile", "", "Optional path to bearer token file to use for -remoteRead.url.")

	idleConnectionTimeout = flag.Duration("remoteRead.idleConnTimeout", 50*time.Second, `Defines a duration for idle (keep-alive connections) to exist. Consider settings this value less to the value of "-http.idleConnTimeout". It must prevent possible "write: broken pipe" and "read: connection reset by peer" errors.`)

	tlsInsecureSkipVerify = flag.Bool("remoteRead.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -remoteRead.url")
	tlsCertFile           = flag.String("remoteRead.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -remoteRead.url")
	tlsKeyFile            = flag.String("remoteRead.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -remoteRead.url")
	tlsCAFile             = flag.String("remoteRead.tlsCAFile", "", "Optional path to TLS CA file to use for verifying connections to -remoteRead.url. "+
		"By default, system CA is used")
	tlsServerName = flag.String("remoteRead.tlsServerName", "", "Optional TLS server name to use for connections to -remoteRead.url. "+
		"By default, the server name from -remoteRead.url is used")

	oauth2ClientID         = flag.String("remoteRead.oauth2.clientID", "", "Optional OAuth2 clientID to use for -remoteRead.url.")
	oauth2ClientSecret     = flag.String("remoteRead.oauth2.clientSecret", "", "Optional OAuth2 clientSecret to use for -remoteRead.url.")
	oauth2ClientSecretFile = flag.String("remoteRead.oauth2.clientSecretFile", "", "Optional OAuth2 clientSecretFile to use for -remoteRead.url.")
	oauth2EndpointParams   = flag.String("remoteRead.oauth2.endpointParams", "", "Optional OAuth2 endpoint parameters to use for -remoteRead.url . "+
		`The endpoint parameters must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}`)
	oauth2TokenURL = flag.String("remoteRead.oauth2.tokenUrl", "", "Optional OAuth2 tokenURL to use for -remoteRead.url. ")
	oauth2Scopes   = flag.String("remoteRead.oauth2.scopes", "", "Optional OAuth2 scopes to use for -remoteRead.url. Scopes must be delimited by ';'.")
)

// InitSecretFlags must be called after flag.Parse and before any logging
func InitSecretFlags() {
	if !*showRemoteReadURL {
		flagutil.RegisterSecretFlag("remoteRead.url")
	}
}

// Init creates a Querier from provided flag values.
// Returns nil if addr flag wasn't set.
func Init() (datasource.QuerierBuilder, error) {
	if *addr == "" {
		return nil, nil
	}
	tr, err := httputils.Transport(*addr, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}
	tr.IdleConnTimeout = *idleConnectionTimeout
	tr.DialContext = httputils.GetStatDialFunc("vmalert_remoteread")

	endpointParams, err := flagutil.ParseJSONMap(*oauth2EndpointParams)
	if err != nil {
		return nil, fmt.Errorf("cannot parse JSON for -remoteRead.oauth2.endpointParams=%s: %w", *oauth2EndpointParams, err)
	}
	authCfg, err := utils.AuthConfig(
		utils.WithBasicAuth(*basicAuthUsername, *basicAuthPassword, *basicAuthPasswordFile),
		utils.WithBearer(*bearerToken, *bearerTokenFile),
		utils.WithOAuth(*oauth2ClientID, *oauth2ClientSecret, *oauth2ClientSecretFile, *oauth2TokenURL, *oauth2Scopes, endpointParams),
		utils.WithHeaders(*headers))
	if err != nil {
		return nil, fmt.Errorf("failed to configure auth: %w", err)
	}
	c := &http.Client{Transport: tr}
	return datasource.NewVMStorage(*addr, authCfg, 0, false, c), nil
}
