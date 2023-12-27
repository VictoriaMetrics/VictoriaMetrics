package notifier

import (
	"flag"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

var (
	configPath                    = flag.String("notifier.config", "", "Path to configuration file for notifiers")
	suppressDuplicateTargetErrors = flag.Bool("notifier.suppressDuplicateTargetErrors", false, "Whether to suppress 'duplicate target' errors during discovery")

	addrs = flagutil.NewArrayString("notifier.url", "Prometheus Alertmanager URL, e.g. http://127.0.0.1:9093. "+
		"List all Alertmanager URLs if it runs in the cluster mode to ensure high availability.")
	showNotifierURL = flag.Bool("notifier.showURL", false, "Whether to avoid stripping sensitive information such as passwords from URL in log messages or UI for -notifier.url. "+
		"It is hidden by default, since it can contain sensitive info such as auth key")
	blackHole = flag.Bool("notifier.blackhole", false, "Whether to blackhole alerting notifications. "+
		"Enable this flag if you want vmalert to evaluate alerting rules without sending any notifications to external receivers (eg. alertmanager). "+
		"-notifier.url, -notifier.config and -notifier.blackhole are mutually exclusive.")

	basicAuthUsername     = flagutil.NewArrayString("notifier.basicAuth.username", "Optional basic auth username for -notifier.url")
	basicAuthPassword     = flagutil.NewArrayString("notifier.basicAuth.password", "Optional basic auth password for -notifier.url")
	basicAuthPasswordFile = flagutil.NewArrayString("notifier.basicAuth.passwordFile", "Optional path to basic auth password file for -notifier.url")

	bearerToken     = flagutil.NewArrayString("notifier.bearerToken", "Optional bearer token for -notifier.url")
	bearerTokenFile = flagutil.NewArrayString("notifier.bearerTokenFile", "Optional path to bearer token file for -notifier.url")

	tlsInsecureSkipVerify = flagutil.NewArrayBool("notifier.tlsInsecureSkipVerify", "Whether to skip tls verification when connecting to -notifier.url")
	tlsCertFile           = flagutil.NewArrayString("notifier.tlsCertFile", "Optional path to client-side TLS certificate file to use when connecting to -notifier.url")
	tlsKeyFile            = flagutil.NewArrayString("notifier.tlsKeyFile", "Optional path to client-side TLS certificate key to use when connecting to -notifier.url")
	tlsCAFile             = flagutil.NewArrayString("notifier.tlsCAFile", "Optional path to TLS CA file to use for verifying connections to -notifier.url. "+
		"By default, system CA is used")
	tlsServerName = flagutil.NewArrayString("notifier.tlsServerName", "Optional TLS server name to use for connections to -notifier.url. "+
		"By default, the server name from -notifier.url is used")

	oauth2ClientID = flagutil.NewArrayString("notifier.oauth2.clientID", "Optional OAuth2 clientID to use for -notifier.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -notifier.url")
	oauth2ClientSecret = flagutil.NewArrayString("notifier.oauth2.clientSecret", "Optional OAuth2 clientSecret to use for -notifier.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -notifier.url")
	oauth2ClientSecretFile = flagutil.NewArrayString("notifier.oauth2.clientSecretFile", "Optional OAuth2 clientSecretFile to use for -notifier.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -notifier.url")
	oauth2EndpointParams = flagutil.NewArrayString("notifier.oauth2.endpointParams", "Optional OAuth2 endpoint parameters to use for the corresponding -notifier.url . "+
		`The endpoint parameters must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}`)
	oauth2TokenURL = flagutil.NewArrayString("notifier.oauth2.tokenUrl", "Optional OAuth2 tokenURL to use for -notifier.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -notifier.url")
	oauth2Scopes = flagutil.NewArrayString("notifier.oauth2.scopes", "Optional OAuth2 scopes to use for -notifier.url. Scopes must be delimited by ';'. "+
		"If multiple args are set, then they are applied independently for the corresponding -notifier.url")
)

// cw holds a configWatcher for configPath configuration file
// configWatcher provides a list of Notifier objects discovered
// from static config or via service discovery.
// cw is not nil only if configPath is provided.
var cw *configWatcher

// Reload checks the changes in configPath configuration file
// and applies changes if any.
func Reload() error {
	if cw == nil {
		return nil
	}
	return cw.reload(*configPath)
}

var staticNotifiersFn func() []Notifier

var (
	// externalLabels is a global variable for holding external labels configured via flags
	// It is supposed to be inited via Init function only.
	externalLabels map[string]string
	// externalURL is a global variable for holding external URL value configured via flag
	// It is supposed to be inited via Init function only.
	externalURL string
)

// Init returns a function for retrieving actual list of Notifier objects.
// Init works in two mods:
//   - configuration via flags (for backward compatibility). Is always static
//     and don't support live reloads.
//   - configuration via file. Supports live reloads and service discovery.
//
// Init returns an error if both mods are used.
func Init(gen AlertURLGenerator, extLabels map[string]string, extURL string) (func() []Notifier, error) {
	externalURL = extURL
	externalLabels = extLabels
	eu, err := url.Parse(externalURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse external URL: %w", err)
	}

	templates.UpdateWithFuncs(templates.FuncsWithExternalURL(eu))

	if *blackHole {
		if len(*addrs) > 0 || *configPath != "" {
			return nil, fmt.Errorf("only one of -notifier.blackhole, -notifier.url and -notifier.config flags must be specified")
		}

		staticNotifiersFn = func() []Notifier {
			return []Notifier{newBlackHoleNotifier()}
		}
		return staticNotifiersFn, nil
	}

	if *configPath == "" && len(*addrs) == 0 {
		return nil, nil
	}
	if *configPath != "" && len(*addrs) > 0 {
		return nil, fmt.Errorf("only one of -notifier.config or -notifier.url flags must be specified")
	}

	if len(*addrs) > 0 {
		notifiers, err := notifiersFromFlags(gen)
		if err != nil {
			return nil, fmt.Errorf("failed to create notifier from flag values: %w", err)
		}
		staticNotifiersFn = func() []Notifier {
			return notifiers
		}
		return staticNotifiersFn, nil
	}

	cw, err = newWatcher(*configPath, gen)
	if err != nil {
		return nil, fmt.Errorf("failed to init config watcher: %w", err)
	}
	return cw.notifiers, nil
}

// InitSecretFlags must be called after flag.Parse and before any logging
func InitSecretFlags() {
	if !*showNotifierURL {
		flagutil.RegisterSecretFlag("notifier.url")
	}
}

func notifiersFromFlags(gen AlertURLGenerator) ([]Notifier, error) {
	var notifiers []Notifier
	for i, addr := range *addrs {
		endpointParamsJSON := oauth2EndpointParams.GetOptionalArg(i)
		endpointParams, err := flagutil.ParseJSONMap(endpointParamsJSON)
		if err != nil {
			return nil, fmt.Errorf("cannot parse JSON for -notifier.oauth2.endpointParams=%s: %w", endpointParamsJSON, err)
		}
		authCfg := promauth.HTTPClientConfig{
			TLSConfig: &promauth.TLSConfig{
				CAFile:             tlsCAFile.GetOptionalArg(i),
				CertFile:           tlsCertFile.GetOptionalArg(i),
				KeyFile:            tlsKeyFile.GetOptionalArg(i),
				ServerName:         tlsServerName.GetOptionalArg(i),
				InsecureSkipVerify: tlsInsecureSkipVerify.GetOptionalArg(i),
			},
			BasicAuth: &promauth.BasicAuthConfig{
				Username:     basicAuthUsername.GetOptionalArg(i),
				Password:     promauth.NewSecret(basicAuthPassword.GetOptionalArg(i)),
				PasswordFile: basicAuthPasswordFile.GetOptionalArg(i),
			},
			BearerToken:     promauth.NewSecret(bearerToken.GetOptionalArg(i)),
			BearerTokenFile: bearerTokenFile.GetOptionalArg(i),
			OAuth2: &promauth.OAuth2Config{
				ClientID:         oauth2ClientID.GetOptionalArg(i),
				ClientSecret:     promauth.NewSecret(oauth2ClientSecret.GetOptionalArg(i)),
				ClientSecretFile: oauth2ClientSecretFile.GetOptionalArg(i),
				EndpointParams:   endpointParams,
				Scopes:           strings.Split(oauth2Scopes.GetOptionalArg(i), ";"),
				TokenURL:         oauth2TokenURL.GetOptionalArg(i),
			},
		}

		addr = strings.TrimSuffix(addr, "/")
		am, err := NewAlertManager(addr+alertManagerPath, gen, authCfg, nil, time.Second*10)
		if err != nil {
			return nil, err
		}
		notifiers = append(notifiers, am)
	}
	return notifiers, nil
}

// Target represents a Notifier and optional
// list of labels added during discovery.
type Target struct {
	Notifier
	Labels *promutils.Labels
}

// TargetType defines how the Target was discovered
type TargetType string

const (
	// TargetStatic is for targets configured statically
	TargetStatic TargetType = "static"
	// TargetConsul is for targets discovered via Consul
	TargetConsul TargetType = "consulSD"
	// TargetDNS is for targets discovered via DNS
	TargetDNS TargetType = "DNSSD"
)

// GetTargets returns list of static or discovered targets
// via notifier configuration.
func GetTargets() map[TargetType][]Target {
	var targets = make(map[TargetType][]Target)

	if staticNotifiersFn != nil {
		for _, ns := range staticNotifiersFn() {
			targets[TargetStatic] = append(targets[TargetStatic], Target{
				Notifier: ns,
			})
		}
	}

	if cw != nil {
		cw.targetsMu.RLock()
		for key, ns := range cw.targets {
			targets[key] = append(targets[key], ns...)
		}
		cw.targetsMu.RUnlock()
	}
	return targets
}
