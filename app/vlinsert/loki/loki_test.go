package loki

import (
	"net/http"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func Test_getTenantIDFromRequest(t *testing.T) {
	f := func(tenant string, expected logstorage.TenantID) {
		t.Helper()
		r, err := http.NewRequest("GET", "http://localhost", nil)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
			return
		}

		r.Header.Set("X-Scope-OrgID", tenant)

		got, err := getTenantIDFromRequest(r)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
			return
		}

		if got.String() != expected.String() {
			t.Fatalf("expected %v, got %v", expected, got)
		}
	}

	f("", logstorage.TenantID{})
	f("123", logstorage.TenantID{AccountID: 123})
	f("123:456", logstorage.TenantID{AccountID: 123, ProjectID: 456})
	f("123:", logstorage.TenantID{AccountID: 123})
	f(":456", logstorage.TenantID{ProjectID: 456})
}
