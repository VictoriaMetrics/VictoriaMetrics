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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

var (
	configPath                    = flag.String("notifier.config", "", "Path to configuration file for notifiers")
	suppressDuplicateTargetErrors = flag.Bool("notifier.suppressDuplicateTargetErrors", false, "Whether to suppress 'duplicate target' errors during discovery")

	addrs = flagutil.NewArray("notifier.url", "Prometheus alertmanager URL, e.g. http://127.0.0.1:9093")

	basicAuthUsername     = flagutil.NewArray("notifier.basicAuth.username", "Optional basic auth username for -notifier.url")
	basicAuthPassword     = flagutil.NewArray("notifier.basicAuth.password", "Optional basic auth password for -notifier.url")
	basicAuthPasswordFile = flagutil.NewArray("notifier.basicAuth.passwordFile", "Optional path to basic auth password file for -notifier.url")

	bearerToken     = flagutil.NewArray("notifier.bearerToken", "Optional bearer token for -notifier.url")
	bearerTokenFile = flagutil.NewArray("notifier.bearerTokenFile", "Optional path to bearer token file for -notifier.url")

	tlsInsecureSkipVerify = flagutil.NewArrayBool("notifier.tlsInsecureSkipVerify", "Whether to skip tls verification when connecting to -notifier.url")
	tlsCertFile           = flagutil.NewArray("notifier.tlsCertFile", "Optional path to client-side TLS certificate file to use when connecting to -notifier.url")
	tlsKeyFile            = flagutil.NewArray("notifier.tlsKeyFile", "Optional path to client-side TLS certificate key to use when connecting to -notifier.url")
	tlsCAFile             = flagutil.NewArray("notifier.tlsCAFile", "Optional path to TLS CA file to use for verifying connections to -notifier.url. "+
		"By default system CA is used")
	tlsServerName = flagutil.NewArray("notifier.tlsServerName", "Optional TLS server name to use for connections to -notifier.url. "+
		"By default the server name from -notifier.url is used")

	oauth2ClientID = flagutil.NewArray("notifier.oauth2.clientID", "Optional OAuth2 clientID to use for -notifier.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -notifier.url")
	oauth2ClientSecret = flagutil.NewArray("notifier.oauth2.clientSecret", "Optional OAuth2 clientSecret to use for -notifier.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -notifier.url")
	oauth2ClientSecretFile = flagutil.NewArray("notifier.oauth2.clientSecretFile", "Optional OAuth2 clientSecretFile to use for -notifier.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -notifier.url")
	oauth2TokenURL = flagutil.NewArray("notifier.oauth2.tokenUrl", "Optional OAuth2 tokenURL to use for -notifier.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -notifier.url")
	oauth2Scopes = flagutil.NewArray("notifier.oauth2.scopes", "Optional OAuth2 scopes to use for -notifier.url. Scopes must be delimited by ';'. "+
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
	if externalLabels != nil || externalURL != "" {
		return nil, fmt.Errorf("BUG: notifier.Init was called multiple times")
	}

	externalURL = extURL
	externalLabels = extLabels
	eu, err := url.Parse(externalURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse external URL: %s", err)
	}

	templates.UpdateWithFuncs(templates.FuncsWithExternalURL(eu))

	if *configPath == "" && len(*addrs) == 0 {
		return nil, nil
	}
	if *configPath != "" && len(*addrs) > 0 {
		return nil, fmt.Errorf("only one of -notifier.config or -notifier.url flags must be specified")
	}

	if len(*addrs) > 0 {
		notifiers, err := notifiersFromFlags(gen)
		if err != nil {
			return nil, fmt.Errorf("failed to create notifier from flag values: %s", err)
		}
		staticNotifiersFn = func() []Notifier {
			return notifiers
		}
		return staticNotifiersFn, nil
	}

	cw, err = newWatcher(*configPath, gen)
	if err != nil {
		return nil, fmt.Errorf("failed to init config watcher: %s", err)
	}
	return cw.notifiers, nil
}

func notifiersFromFlags(gen AlertURLGenerator) ([]Notifier, error) {
	var notifiers []Notifier
	for i, addr := range *addrs {
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
	Labels []prompbmarshal.Label
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
