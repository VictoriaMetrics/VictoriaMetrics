package tests

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/google/go-cmp/cmp"
)

func TestClusterTenantsSearch(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	const numDays = 100
	now := time.Now().UTC()
	samples := make([]string, numDays)
	tenantIDs := make([]string, numDays)
	for i := range numDays {
		d := now.Add(-time.Duration(i) * 24 * time.Hour)
		metricName := fmt.Sprintf("metric_%04d_%02d_%02d", d.Year(), d.Month(), d.Day())
		accountID := fmt.Sprintf("%d", d.Year())
		projectID := fmt.Sprintf("%d%d", d.Month(), d.Day())
		value := i
		timestamp := d.UnixMilli()
		samples[i] = fmt.Sprintf(`%s{vm_account_id="%s", vm_project_id="%s"} %d %d`, metricName, accountID, projectID, value, timestamp)
		tenantIDs[i] = fmt.Sprintf("%s:%s", accountID, projectID)
	}

	sut := tc.MustStartDefaultCluster()
	sut.PrometheusAPIV1ImportPrometheus(t, samples, apptest.QueryOpts{Tenant: "multitenant"})
	sut.ForceFlush(t)

	f := func(start, end *time.Time, wantTenants []string) {
		t.Helper()
		var opts apptest.QueryOpts
		if start != nil {
			opts.Start = fmt.Sprintf("%d", start.UnixMilli())
		}
		if end != nil {
			opts.End = fmt.Sprintf("%d", end.UnixMilli())
		}
		got := sut.APIV1AdminTenants(t, opts)
		wantData := slices.Clone(wantTenants)
		slices.Sort(wantData)
		want := &apptest.AdminTenantsResponse{
			Status: "success",
			Data:   wantData,
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected response (-want, +got):\n%s", diff)
		}
	}

	var start, end time.Time

	// Get tenants for unset start and end params.
	f(nil, nil, tenantIDs[:1])

	// Get tenants for unset start and yesterday end.
	end = now.Add(-24 * time.Hour)
	f(nil, &end, tenantIDs[1:])

	// Get tenants for zero start and yesterday end.
	start = time.UnixMilli(0)
	end = now.Add(-24 * time.Hour)
	f(&start, &end, tenantIDs[1:])

	// Get tenants for first Unix milli start and yesterday end.
	start = time.UnixMilli(1)
	end = now.Add(-24 * time.Hour)
	f(&start, &end, tenantIDs[1:])

	// Get tenants for first last milli of the first Unix day start and
	// yesterday end.
	start = time.UnixMilli(24*3600*1000 - 1)
	end = now.Add(-24 * time.Hour)
	f(&start, &end, tenantIDs[1:])

	// Get tenants for yesterday start and unset end.
	// By default, end is zero, thus the time range is incorrect and empty
	// result is expected.
	start = now.Add(-24 * time.Hour)
	f(&start, nil, tenantIDs[:0])

	// Get tenants for yesterday start and zero end.
	// The time range is incorrect and empty result is expected.
	start = now.Add(-24 * time.Hour)
	end = time.UnixMilli(0)
	f(&start, &end, tenantIDs[:0])

	// Get tenants for current day.
	start = now
	end = now
	f(&start, &end, tenantIDs[:1])

	// Get tenants for the last 2 days.
	start = now.Add(-24 * time.Hour)
	end = now
	f(&start, &end, tenantIDs[:2])

	// Get tenants for last 30 days
	start = now.Add(-29 * 24 * time.Hour)
	end = now
	f(&start, &end, tenantIDs[:30])

	// Get tenants for last 60 numDays
	start = now.Add(-59 * 24 * time.Hour)
	end = now
	f(&start, &end, tenantIDs[:60])
}
