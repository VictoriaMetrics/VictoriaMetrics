package tests

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestSingleMetricsMetadataRestart(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	path := filepath.Join(t.TempDir(), "storage")
	flags := []string{
		"-storageDataPath=" + path,
		"-memory.allowedBytes=128MiB",
	}
	sut := tc.MustStartVmsingle("vmsingle", flags)
	sut.PrometheusAPIV1Write(t, prompb.WriteRequest{
		Metadata: []prompb.MetricMetadata{
			{MetricFamilyName: "requests", Help: "requests received", Unit: "requests", Type: prompb.MetricTypeCounter},
			{MetricFamilyName: "duration", Help: "request duration", Unit: "seconds", Type: prompb.MetricTypeGauge},
		},
	}, apptest.QueryOpts{})
	want := &apptest.PrometheusAPIV1Metadata{
		Status: "success",
		Data: map[string][]apptest.MetadataEntry{
			"requests": {{Help: "requests received", Unit: "requests", Type: "counter"}},
			"duration": {{Help: "request duration", Unit: "seconds", Type: "gauge"}},
		},
	}
	check := func() {
		t.Helper()
		got := sut.PrometheusAPIV1Metadata(t, "", 0, apptest.QueryOpts{})
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("unexpected metadata (-want, +got):\n%s", diff)
		}
	}
	check()
	// Do not flush or read between this partial update and shutdown.
	sut.PrometheusAPIV1Write(t, prompb.WriteRequest{
		Metadata: []prompb.MetricMetadata{{MetricFamilyName: "requests", Help: "updated help"}},
	}, apptest.QueryOpts{})
	want.Data["requests"][0].Help = "updated help"
	for range 2 {
		tc.StopApp("vmsingle")
		sut = tc.MustStartVmsingle("vmsingle", flags)
		check()
	}
}
