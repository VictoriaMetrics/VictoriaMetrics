package notifier

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

// AlertManager represents integration provider with Prometheus alert manager
// https://github.com/prometheus/alertmanager
type AlertManager struct {
	addr    *url.URL
	argFunc AlertURLGenerator
	client  *http.Client
	timeout time.Duration

	authCfg *promauth.Config
	// stores already parsed RelabelConfigs object
	relabelConfigs *promrelabel.ParsedConfigs

	metrics *metrics
}

type metrics struct {
	alertsSent       *utils.Counter
	alertsSendErrors *utils.Counter
}

func newMetrics(addr string) *metrics {
	return &metrics{
		alertsSent:       utils.GetOrCreateCounter(fmt.Sprintf("vmalert_alerts_sent_total{addr=%q}", addr)),
		alertsSendErrors: utils.GetOrCreateCounter(fmt.Sprintf("vmalert_alerts_send_errors_total{addr=%q}", addr)),
	}
}

// Close is a destructor method for AlertManager
func (am *AlertManager) Close() {
	am.metrics.alertsSent.Unregister()
	am.metrics.alertsSendErrors.Unregister()
}

// Addr returns address where alerts are sent.
func (am AlertManager) Addr() string {
	if *showNotifierURL {
		return am.addr.String()
	}
	return am.addr.Redacted()
}

// Send an alert or resolve message
func (am *AlertManager) Send(ctx context.Context, alerts []Alert, headers map[string]string) error {
	am.metrics.alertsSent.Add(len(alerts))
	err := am.send(ctx, alerts, headers)
	if err != nil {
		am.metrics.alertsSendErrors.Add(len(alerts))
	}
	return err
}

func (am *AlertManager) send(ctx context.Context, alerts []Alert, headers map[string]string) error {
	b := &bytes.Buffer{}
	writeamRequest(b, alerts, am.argFunc, am.relabelConfigs)

	req, err := http.NewRequest(http.MethodPost, am.addr.String(), b)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	if am.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, am.timeout)
		defer cancel()
	}

	req = req.WithContext(ctx)

	if am.authCfg != nil {
		err = am.authCfg.SetHeaders(req, true)
		if err != nil {
			return err
		}
	}
	resp, err := am.client.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	amURL := am.addr.Redacted()
	if *showNotifierURL {
		amURL = am.addr.String()
	}
	if resp.StatusCode/100 != 2 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response from %q: %w", amURL, err)
		}
		return fmt.Errorf("invalid SC %d from %q; response body: %s", resp.StatusCode, amURL, string(body))
	}
	return nil
}

// AlertURLGenerator returns URL to single alert by given name
type AlertURLGenerator func(Alert) string

const alertManagerPath = "/api/v2/alerts"

// NewAlertManager is a constructor for AlertManager
func NewAlertManager(alertManagerURL string, fn AlertURLGenerator, authCfg promauth.HTTPClientConfig,
	relabelCfg *promrelabel.ParsedConfigs, timeout time.Duration,
) (*AlertManager, error) {
	tls := &promauth.TLSConfig{}
	if authCfg.TLSConfig != nil {
		tls = authCfg.TLSConfig
	}
	tr, err := httputils.Transport(alertManagerURL, tls.CertFile, tls.KeyFile, tls.CAFile, tls.ServerName, tls.InsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	ba := new(promauth.BasicAuthConfig)
	oauth := new(promauth.OAuth2Config)
	if authCfg.BasicAuth != nil {
		ba = authCfg.BasicAuth
	}
	if authCfg.OAuth2 != nil {
		oauth = authCfg.OAuth2
	}

	aCfg, err := utils.AuthConfig(
		utils.WithBasicAuth(ba.Username, ba.Password.String(), ba.PasswordFile),
		utils.WithBearer(authCfg.BearerToken.String(), authCfg.BearerTokenFile),
		utils.WithOAuth(oauth.ClientID, oauth.ClientSecret.String(), oauth.ClientSecretFile, oauth.TokenURL, strings.Join(oauth.Scopes, ";"), oauth.EndpointParams))
	if err != nil {
		return nil, fmt.Errorf("failed to configure auth: %w", err)
	}

	amURL, err := url.Parse(alertManagerURL)
	if err != nil {
		return nil, fmt.Errorf("provided incorrect notifier url: %w", err)
	}
	if !*showNotifierURL {
		alertManagerURL = amURL.Redacted()
	}
	return &AlertManager{
		addr:           amURL,
		argFunc:        fn,
		authCfg:        aCfg,
		relabelConfigs: relabelCfg,
		client:         &http.Client{Transport: tr},
		timeout:        timeout,
		metrics:        newMetrics(alertManagerURL),
	}, nil
}
