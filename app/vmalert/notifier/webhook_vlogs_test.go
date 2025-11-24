package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestWebhookVLogs_Send(t *testing.T) {
	// Mock VictoriaLogs
	vlogsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("expected path /, got %s", r.URL.Path)
		}
		query := r.URL.Query().Get("query")
		if query != "test_query" {
			t.Errorf("expected query test_query, got %s", query)
		}
		// Return mock logs
		w.Write([]byte(`{"_msg": "log line 1"}{"_msg": "log line 2"}`))
	}))
	defer vlogsServer.Close()

	// Mock Slack
	slackServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Text        string `json:"text"`
			Attachments []struct {
				Text string `json:"text"`
			} `json:"attachments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode slack payload: %s", err)
		}
		if payload.Text != "[FIRING] TestAlert" {
			t.Errorf("expected text [FIRING] TestAlert, got %s", payload.Text)
		}
		expectedDesc := "Test Description\n<http://ingress/vmalert?query=test_query|See Logs in VMUI>\n*Recent Logs:*\n• `log line 1`\n• `log line 2`\n"
		if payload.Attachments[0].Text != expectedDesc {
			t.Errorf("expected attachment text %q, got %q", expectedDesc, payload.Attachments[0].Text)
		}
	}))
	defer slackServer.Close()

	w := NewWebhookVLogs(vlogsServer.URL, slackServer.URL, "http://ingress/vmalert")

	alerts := []Alert{
		{
			Name: "TestAlert",
			Annotations: map[string]string{
				"query":       "test_query",
				"description": "Test Description",
			},
			Labels: map[string]string{
				"alertname": "TestAlert",
			},
			State: StateFiring,
		},
	}

	if err := w.Send(context.Background(), alerts, [][]prompb.Label{{}}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}
