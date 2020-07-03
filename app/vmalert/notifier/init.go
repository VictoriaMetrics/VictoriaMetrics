package notifier

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
)

var (
	addrs             = flagutil.NewArray("notifier.url", "Prometheus alertmanager URL. Required parameter. e.g. http://127.0.0.1:9093")
	basicAuthUsername = flagutil.NewArray("notifier.basicAuth.username", "Optional basic auth username for -datasource.url")
	basicAuthPassword = flagutil.NewArray("notifier.basicAuth.password", "Optional basic auth password for -datasource.url")

	tlsInsecureSkipVerify = flag.Bool("notifier.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -notifier.url")
	tlsCertFile           = flagutil.NewArray("notifier.tlsCertFile", "Optional path to client-side TLS certificate file to use when connecting to -notifier.url")
	tlsKeyFile            = flagutil.NewArray("notifier.tlsKeyFile", "Optional path to client-side TLS certificate key to use when connecting to -notifier.url")
	tlsCAFile             = flagutil.NewArray("notifier.tlsCAFile", "Optional path to TLS CA file to use for verifying connections to -notifier.url. "+
		"By default system CA is used")
	tlsServerName = flagutil.NewArray("notifier.tlsServerName", "Optional TLS server name to use for connections to -notifier.url. "+
		"By default the server name from -notifier.url is used")
)

// Init creates a Notifier object based on provided flags.
func Init(gen AlertURLGenerator) ([]Notifier, error) {
	if len(*addrs) == 0 {
		flag.PrintDefaults()
		return nil, fmt.Errorf("at least one `-notifier.url` must be set")
	}

	var notifiers []Notifier
	for i, addr := range *addrs {
		cert, key := tlsCertFile.GetOptionalArg(i), tlsKeyFile.GetOptionalArg(i)
		ca, serverName := tlsCAFile.GetOptionalArg(i), tlsServerName.GetOptionalArg(i)
		tr, err := utils.Transport(addr, cert, key, ca, serverName, *tlsInsecureSkipVerify)
		if err != nil {
			return nil, fmt.Errorf("failed to create transport: %w", err)
		}
		user, pass := basicAuthUsername.GetOptionalArg(i), basicAuthPassword.GetOptionalArg(i)
		am := NewAlertManager(addr, user, pass, gen, &http.Client{Transport: tr})
		notifiers = append(notifiers, am)
	}

	return notifiers, nil
}
