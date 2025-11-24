package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// WebhookVLogs represents a notifier that queries VictoriaLogs and sends alerts to Slack
type WebhookVLogs struct {
	vlogsURL    string
	slackURL    string
	vlogIngress string
	client      *http.Client
	lastError   string
	mu          sync.Mutex
}

// NewWebhookVLogs creates a new WebhookVLogs notifier
func NewWebhookVLogs(vlogsURL, slackURL, vlogIngress string) *WebhookVLogs {
	return &WebhookVLogs{
		vlogsURL:    strings.TrimSuffix(vlogsURL, "/"),
		slackURL:    slackURL,
		vlogIngress: strings.TrimSuffix(vlogIngress, "/"),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Close is a destructor for the Notifier
func (w *WebhookVLogs) Close() {}

// Addr returns address where alerts are sent
func (w *WebhookVLogs) Addr() string {
	return w.slackURL
}

// LastError returns error, that occured during last attempt to send data
func (w *WebhookVLogs) LastError() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastError
}

// Send sends the given list of alerts
func (w *WebhookVLogs) Send(ctx context.Context, alerts []Alert, alertLabels [][]prompb.Label, notifierHeaders map[string]string) error {
	var firstErr error
	w.mu.Lock()
	w.lastError = ""
	w.mu.Unlock()

	for _, alert := range alerts {
		// Check for query annotation
		query, ok := alert.Annotations["query"]
		if !ok || query == "" {
			continue
		}

		logs, logURL, err := w.queryVLogs(ctx, query)
		if err != nil {
			logger.Errorf("failed to query VictoriaLogs for alert %q: %s", alert.Name, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		if err := w.sendSlack(ctx, alert, logs, logURL); err != nil {
			logger.Errorf("failed to send Slack message for alert %q: %s", alert.Name, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	if firstErr != nil {
		w.mu.Lock()
		w.lastError = firstErr.Error()
		w.mu.Unlock()
	}
	return firstErr
}

func (w *WebhookVLogs) queryVLogs(ctx context.Context, query string) ([]string, string, error) {
	params := url.Values{}
	params.Set("query", query)
	fullURL := fmt.Sprintf("%s?%s", w.vlogsURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, "", err
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	var logs []string
	decoder := json.NewDecoder(bytes.NewReader(body))

	// Define a struct to match VictoriaLogs query result
	type queryResult struct {
		Msg string `json:"_msg"`
	}

	for decoder.More() {
		var result queryResult
		if err := decoder.Decode(&result); err != nil {
			return nil, "", err
		}
		logs = append(logs, result.Msg)
	}

	ingressURL := ""
	if w.vlogIngress != "" {
		ingressURL = fmt.Sprintf("%s?%s", w.vlogIngress, params.Encode())
	}

	return logs, ingressURL, nil
}

func (w *WebhookVLogs) sendSlack(ctx context.Context, alert Alert, logs []string, logURL string) error {
	// Slack payload
	type Field struct {
		Title string `json:"title"`
		Value string `json:"value"`
		Short bool   `json:"short"`
	}
	type Attachment struct {
		Color  string  `json:"color,omitempty"`
		Text   string  `json:"text,omitempty"`
		Fields []Field `json:"fields,omitempty"`
	}
	type Payload struct {
		Text        string       `json:"text,omitempty"`
		Attachments []Attachment `json:"attachments,omitempty"`
	}

	attachment := Attachment{
		Fields: []Field{},
	}

	for k, v := range alert.Labels {
		attachment.Fields = append(attachment.Fields, Field{
			Title: k,
			Value: v,
			Short: true,
		})
	}

	desc := alert.Annotations["description"]
	var sb strings.Builder
	sb.WriteString(desc)
	if logURL != "" {
		sb.WriteString(fmt.Sprintf("\n<%s|See Logs in VMUI>", logURL))
	}
	if len(logs) > 0 {
		sb.WriteString("\n*Recent Logs:*\n")
		max := 20
		if len(logs) < max {
			max = len(logs)
		}
		for _, line := range logs[:max] {
			sb.WriteString(fmt.Sprintf("• `%s`\n", line))
		}
	}
	attachment.Text = sb.String()

	color := "danger"
	if alert.State == StateInactive {
		color = "good"
	}

	attachment.Color = color

	payload := Payload{
		Text:        fmt.Sprintf("[%s] %s", "FIRING", alert.Name), // Placeholder for status
		Attachments: []Attachment{attachment},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.slackURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("slack error: %d %s", resp.StatusCode, string(body))
	}

	return nil
}
