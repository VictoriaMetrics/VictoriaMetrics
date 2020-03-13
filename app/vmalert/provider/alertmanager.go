package provider

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

const alertsPath = "/api/v2/alerts"

var pool = sync.Pool{New: func() interface{} {
	return &bytes.Buffer{}
}}

// AlertManager represents integration provider with Prometheus alert manager
type AlertManager struct {
	alertURL string
	argFunc  AlertURLGenerator
	client   *http.Client
}

// AlertURLGenerator returns URL to single alert by given name
type AlertURLGenerator func(group, name string) string

// NewAlertManager is a constructor for AlertManager
func NewAlertManager(alertManagerURL string, fn AlertURLGenerator, c *http.Client) *AlertManager {
	return &AlertManager{
		alertURL: strings.TrimSuffix(alertManagerURL, "/") + alertsPath,
		argFunc:  fn,
		client:   c,
	}
}

// Send an alert or resolve message
func (am *AlertManager) Send(alerts []Alert) error {
	b := pool.Get().(*bytes.Buffer)
	b.Reset()
	defer pool.Put(b)
	writeamRequest(b, alerts, am.argFunc)
	resp, err := am.client.Post(am.alertURL, "application/json", b)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		b.Reset()
		if _, err := io.Copy(b, resp.Body); err != nil {
			logger.Errorf("unable to copy error response body to buffer %s", err)
		}
		return fmt.Errorf("invalid response from alertmanager %s", b)
	}
	return nil
}
