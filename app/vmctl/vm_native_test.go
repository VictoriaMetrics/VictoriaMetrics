package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/native"
	remote_read_integration "github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/testdata/servers_integration_test"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/promql"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

const (
	storagePath     = "TestStorage"
	retentionPeriod = "100y"
)

func TestVMNativeProcessorRun(t *testing.T) {
	f := func(startStr, endStr string, numOfSeries, numOfSamples int, resultExpected []vm.TimeSeries) {
		t.Helper()

		src := remote_read_integration.NewRemoteWriteServer(t)
		dst := remote_read_integration.NewRemoteWriteServer(t)

		defer func() {
			src.Close()
			dst.Close()
		}()

		start, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			t.Fatalf("cannot parse start time: %s", err)
		}

		end, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			t.Fatalf("cannot parse end time: %s", err)
		}

		matchName := "__name__"
		matchValue := ".*"
		filter := native.Filter{
			Match:     fmt.Sprintf("{%s=~%q}", matchName, matchValue),
			TimeStart: startStr,
			TimeEnd:   endStr,
		}

		rws := remote_read_integration.GenerateVNSeries(start.Unix(), end.Unix(), int64(numOfSeries), int64(numOfSamples))

		src.Series(rws)
		dst.ExpectedSeries(resultExpected)

		if err := fillStorage(rws); err != nil {
			t.Fatalf("cannot add series to storage: %s", err)
		}

		srcClient := &native.Client{
			AuthCfg:     nil,
			Addr:        src.URL(),
			ExtraLabels: []string{},
			HTTPClient:  &http.Client{Transport: &http.Transport{DisableKeepAlives: false}},
		}
		dstClient := &native.Client{
			AuthCfg:     nil,
			Addr:        dst.URL(),
			ExtraLabels: []string{},
			HTTPClient:  &http.Client{Transport: &http.Transport{DisableKeepAlives: false}},
		}

		isSilent = true
		defer func() { isSilent = false }()

		bf, err := backoff.New(10, 1.8, time.Second*2)
		if err != nil {
			t.Fatalf("cannot create backoff: %s", err)
		}

		p := &vmNativeProcessor{
			filter:   filter,
			dst:      dstClient,
			src:      srcClient,
			backoff:  bf,
			cc:       1,
			isNative: true,
		}

		ctx := context.Background()
		if err := p.run(ctx); err != nil {
			t.Fatalf("run() error: %s", err)
		}
		deleted, err := deleteSeries(matchName, matchValue)
		if err != nil {
			t.Fatalf("cannot delete series: %s", err)
		}
		if deleted != numOfSeries {
			t.Fatalf("unexpected number of deleted series; got %d; want %d", deleted, numOfSeries)
		}
	}

	processFlags()
	vmstorage.Init(promql.ResetRollupResultCacheIfNeeded)
	defer func() {
		vmstorage.Stop()
		if err := os.RemoveAll(storagePath); err != nil {
			log.Fatalf("cannot remove %q: %s", storagePath, err)
		}
	}()

	barpool.Disable(true)
	defer func() {
		barpool.Disable(false)
	}()

	// step minute on minute time range
	start := "2022-11-25T11:23:05+02:00"
	end := "2022-11-27T11:24:05+02:00"
	numOfSeries := 3
	numOfSamples := 2
	resultExpected := []vm.TimeSeries{
		{
			Name:       "vm_metric_1",
			LabelPairs: []vm.LabelPair{{Name: "job", Value: "0"}},
			Timestamps: []int64{1669368185000, 1669454615000},
			Values:     []float64{0, 0},
		},
		{
			Name:       "vm_metric_1",
			LabelPairs: []vm.LabelPair{{Name: "job", Value: "1"}},
			Timestamps: []int64{1669368185000, 1669454615000},
			Values:     []float64{100, 100},
		},
		{
			Name:       "vm_metric_1",
			LabelPairs: []vm.LabelPair{{Name: "job", Value: "2"}},
			Timestamps: []int64{1669368185000, 1669454615000},
			Values:     []float64{200, 200},
		},
	}
	f(start, end, numOfSeries, numOfSamples, resultExpected)

	// step month on month time range
	start = "2022-09-26T11:23:05+02:00"
	end = "2022-11-26T11:24:05+02:00"
	numOfSeries = 3
	numOfSamples = 2
	resultExpected = []vm.TimeSeries{
		{
			Name:       "vm_metric_1",
			LabelPairs: []vm.LabelPair{{Name: "job", Value: "0"}},
			Timestamps: []int64{1664184185000},
			Values:     []float64{0},
		},
		{
			Name:       "vm_metric_1",
			LabelPairs: []vm.LabelPair{{Name: "job", Value: "0"}},
			Timestamps: []int64{1666819415000},
			Values:     []float64{0},
		},
		{
			Name:       "vm_metric_1",
			LabelPairs: []vm.LabelPair{{Name: "job", Value: "1"}},
			Timestamps: []int64{1664184185000},
			Values:     []float64{100},
		},
		{
			Name:       "vm_metric_1",
			LabelPairs: []vm.LabelPair{{Name: "job", Value: "1"}},
			Timestamps: []int64{1666819415000},
			Values:     []float64{100},
		},
		{
			Name:       "vm_metric_1",
			LabelPairs: []vm.LabelPair{{Name: "job", Value: "2"}},
			Timestamps: []int64{1664184185000},
			Values:     []float64{200},
		},
		{
			Name:       "vm_metric_1",
			LabelPairs: []vm.LabelPair{{Name: "job", Value: "2"}},
			Timestamps: []int64{1666819415000},
			Values:     []float64{200},
		},
	}
	f(start, end, numOfSeries, numOfSamples, resultExpected)
}

func processFlags() {
	flag.Parse()
	for _, fv := range []struct {
		flag  string
		value string
	}{
		{flag: "storageDataPath", value: storagePath},
		{flag: "retentionPeriod", value: retentionPeriod},
	} {
		// panics if flag doesn't exist
		if err := flag.Lookup(fv.flag).Value.Set(fv.value); err != nil {
			log.Fatalf("unable to set %q with value %q, err: %v", fv.flag, fv.value, err)
		}
	}
}

func fillStorage(series []vm.TimeSeries) error {
	var mrs []storage.MetricRow
	for _, series := range series {
		var labels []prompb.Label
		for _, lp := range series.LabelPairs {
			labels = append(labels, prompb.Label{
				Name:  lp.Name,
				Value: lp.Value,
			})
		}
		if series.Name != "" {
			labels = append(labels, prompb.Label{
				Name:  "__name__",
				Value: series.Name,
			})
		}
		mr := storage.MetricRow{}
		mr.MetricNameRaw = storage.MarshalMetricNameRaw(mr.MetricNameRaw[:0], labels)

		timestamps := series.Timestamps
		values := series.Values
		for i, value := range values {
			mr.Timestamp = timestamps[i]
			mr.Value = value
			mrs = append(mrs, mr)
		}
	}

	if err := vmstorage.AddRows(mrs); err != nil {
		return fmt.Errorf("unexpected error in AddRows: %s", err)
	}
	vmstorage.Storage.DebugFlush()
	return nil
}

func deleteSeries(name, value string) (int, error) {
	tfs := storage.NewTagFilters()
	if err := tfs.Add([]byte(name), []byte(value), false, true); err != nil {
		return 0, fmt.Errorf("unexpected error in TagFilters.Add: %w", err)
	}
	return vmstorage.DeleteSeries(nil, []*storage.TagFilters{tfs}, 1e3)
}

func TestBuildMatchWithFilter_Failure(t *testing.T) {
	f := func(filter, metricName string) {
		t.Helper()

		_, err := buildMatchWithFilter(filter, metricName)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// match with error
	f(`{cluster~=".*"}`, "http_request_count_total")
}

func TestBuildMatchWithFilter_Success(t *testing.T) {
	f := func(filter, metricName, resultExpected string) {
		t.Helper()

		result, err := buildMatchWithFilter(filter, metricName)
		if err != nil {
			t.Fatalf("buildMatchWithFilter() error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	// parsed metric with label
	f(`{__name__="http_request_count_total",cluster="kube1"}`, "http_request_count_total", `{cluster="kube1",__name__="http_request_count_total"}`)

	// metric name with label
	f(`http_request_count_total{cluster="kube1"}`, "http_request_count_total", `{cluster="kube1",__name__="http_request_count_total"}`)

	// parsed metric with regexp value
	f(`{__name__="http_request_count_total",cluster=~"kube.*"}`, "http_request_count_total", `{cluster=~"kube.*",__name__="http_request_count_total"}`)

	// only label with regexp
	f(`{cluster=~".*"}`, "http_request_count_total", `{cluster=~".*",__name__="http_request_count_total"}`)

	// many labels in filter with regexp
	f(`{cluster=~".*",job!=""}`, "http_request_count_total", `{cluster=~".*",job!="",__name__="http_request_count_total"}`)

	// all names
	f(`{__name__!=""}`, "http_request_count_total", `{__name__="http_request_count_total"}`)

	// with many underscores labels
	f(`{__name__!="", __meta__!=""}`, "http_request_count_total", `{__meta__!="",__name__="http_request_count_total"}`)

	// metric name has regexp
	f(`{__name__=~".*"}`, "http_request_count_total", `{__name__="http_request_count_total"}`)

	// metric name has negative regexp
	f(`{__name__!~".*"}`, "http_request_count_total", `{__name__="http_request_count_total"}`)
}
