package notifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

// TestSendPerNotifierRelabelDoesNotShareLabels reproduces
// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/11676:
// Send fans the same relabeled label slices out to every active notifier,
// and every notifier applies its own alert_relabel_configs. That relabeling
// must not mutate the shared slice, otherwise concurrently sending notifiers
// observe each other's partially applied relabel chains. The chain is the
// mark-then-append idiom: owner=a alerts get team-a and are marked, unmarked
// alerts get team-b, the mark is dropped and the leading separator trimmed.
func TestSendPerNotifierRelabelDoesNotShareLabels(t *testing.T) {
	rc, err := promrelabel.ParseRelabelConfigsData([]byte(`
- source_labels: [teams, owner]
  separator: ";"
  regex: "(.*);a"
  target_label: teams
  replacement: "${1},team-a"
- source_labels: [teams]
  regex: ".*team-a.*"
  target_label: __tmp_owned
  replacement: "1"
- source_labels: [teams, __tmp_owned]
  separator: ";"
  regex: "(.*);"
  target_label: teams
  replacement: "${1},team-b"
- regex: __tmp_owned
  action: labeldrop
- source_labels: [teams]
  regex: ",(.*)"
  target_label: teams
  replacement: "${1}"
`))
	if err != nil {
		t.Fatalf("unexpected error when parsing relabeling config: %s", err)
	}

	var mu sync.Mutex
	var corrupted []string
	handler := func(_ http.ResponseWriter, r *http.Request) {
		var payload []struct {
			Labels map[string]string `json:"labels"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("cannot unmarshal alertmanager payload: %s", err)
			return
		}
		for _, a := range payload {
			// owner=a alerts are claimed by the first rule and must skip the
			// team-b append; owner=b alerts must take it. Either way the
			// temporary mark is dropped and the leading separator trimmed.
			want := "team-b"
			if a.Labels["owner"] == "a" {
				want = "team-a"
			}
			if a.Labels["teams"] != want || a.Labels["__tmp_owned"] != "" {
				mu.Lock()
				corrupted = append(corrupted, fmt.Sprintf("%v", a.Labels))
				mu.Unlock()
			}
		}
	}

	const notifiersCount = 3
	nts := make([]Notifier, 0, notifiersCount)
	for i := 0; i < notifiersCount; i++ {
		srv := httptest.NewServer(http.HandlerFunc(handler))
		defer srv.Close()
		am, err := NewAlertManager(srv.URL+alertManagerPath, func(Alert) string { return "" }, promauth.HTTPClientConfig{}, rc, 0)
		if err != nil {
			t.Fatalf("unexpected error when creating notifier: %s", err)
		}
		nts = append(nts, am)
	}
	prevGetActiveNotifiers := getActiveNotifiers
	getActiveNotifiers = func() []Notifier { return nts }
	defer func() { getActiveNotifiers = prevGetActiveNotifiers }()

	alerts := make([]Alert, 0, 300)
	for i := 0; i < 300; i++ {
		owner := "a"
		if i%2 == 1 {
			owner = "b"
		}
		alerts = append(alerts, Alert{
			Name:   "test",
			Labels: map[string]string{"alertname": "test", "id": fmt.Sprintf("%d", i), "owner": owner},
		})
	}
	for round := 0; round < 20; round++ {
		for err := range Send(context.Background(), alerts, nil) {
			if err != nil {
				t.Fatalf("unexpected error from Send: %s", err)
			}
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(corrupted) > 0 {
		t.Fatalf("%d alerts reached notifiers with a partially relabeled label set; first: %s", len(corrupted), corrupted[0])
	}
}
