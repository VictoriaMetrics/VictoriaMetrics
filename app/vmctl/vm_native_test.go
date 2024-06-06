package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
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

func Test_vmNativeProcessor_run(t *testing.T) {

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
	defer func() { isSilent = false }()

	type fields struct {
		filter       native.Filter
		dst          *native.Client
		src          *native.Client
		backoff      *backoff.Backoff
		s            *stats
		rateLimit    int64
		interCluster bool
		cc           int
		matchName    string
		matchValue   string
	}
	type args struct {
		ctx    context.Context
		silent bool
	}

	tests := []struct {
		name           string
		fields         fields
		args           args
		vmSeries       func(start, end, numOfSeries, numOfSamples int64) []vm.TimeSeries
		expectedSeries []vm.TimeSeries
		start          string
		end            string
		numOfSamples   int64
		numOfSeries    int64
		chunk          string
		wantErr        bool
	}{
		{
			name:         "step minute on minute time range",
			start:        "2022-11-25T11:23:05+02:00",
			end:          "2022-11-27T11:24:05+02:00",
			numOfSamples: 2,
			numOfSeries:  3,
			chunk:        stepper.StepMinute,
			fields: fields{
				filter:       native.Filter{},
				backoff:      backoff.New(),
				rateLimit:    0,
				interCluster: false,
				cc:           1,
				matchName:    "__name__",
				matchValue:   ".*",
			},
			args: args{
				ctx:    context.Background(),
				silent: true,
			},
			vmSeries: remote_read_integration.GenerateVNSeries,
			expectedSeries: []vm.TimeSeries{
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
			},
			wantErr: false,
		},
		{
			name:         "step month on month time range",
			start:        "2022-09-26T11:23:05+02:00",
			end:          "2022-11-26T11:24:05+02:00",
			numOfSamples: 2,
			numOfSeries:  3,
			chunk:        stepper.StepMonth,
			fields: fields{
				filter:       native.Filter{},
				backoff:      backoff.New(),
				rateLimit:    0,
				interCluster: false,
				cc:           1,
				matchName:    "__name__",
				matchValue:   ".*",
			},
			args: args{
				ctx:    context.Background(),
				silent: true,
			},
			vmSeries: remote_read_integration.GenerateVNSeries,
			expectedSeries: []vm.TimeSeries{
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
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := remote_read_integration.NewRemoteWriteServer(t)
			dst := remote_read_integration.NewRemoteWriteServer(t)

			defer func() {
				src.Close()
				dst.Close()
			}()

			start, err := time.Parse(time.RFC3339, tt.start)
			if err != nil {
				t.Fatalf("Error parse start time: %s", err)
			}

			end, err := time.Parse(time.RFC3339, tt.end)
			if err != nil {
				t.Fatalf("Error parse end time: %s", err)
			}

			tt.fields.filter.Match = fmt.Sprintf("{%s=~%q}", tt.fields.matchName, tt.fields.matchValue)
			tt.fields.filter.TimeStart = tt.start
			tt.fields.filter.TimeEnd = tt.end

			rws := tt.vmSeries(start.Unix(), end.Unix(), tt.numOfSeries, tt.numOfSamples)

			src.Series(rws)
			dst.ExpectedSeries(tt.expectedSeries)

			if err := fillStorage(rws); err != nil {
				t.Fatalf("error add series to storage: %s", err)
			}

			tt.fields.src = &native.Client{
				AuthCfg:     nil,
				Addr:        src.URL(),
				ExtraLabels: []string{},
				HTTPClient:  &http.Client{Transport: &http.Transport{DisableKeepAlives: false}},
			}
			tt.fields.dst = &native.Client{
				AuthCfg:     nil,
				Addr:        dst.URL(),
				ExtraLabels: []string{},
				HTTPClient:  &http.Client{Transport: &http.Transport{DisableKeepAlives: false}},
			}

			isSilent = tt.args.silent
			p := &vmNativeProcessor{
				filter:       tt.fields.filter,
				dst:          tt.fields.dst,
				src:          tt.fields.src,
				backoff:      tt.fields.backoff,
				s:            tt.fields.s,
				rateLimit:    tt.fields.rateLimit,
				interCluster: tt.fields.interCluster,
				cc:           tt.fields.cc,
				isNative:     true,
			}

			if err := p.run(tt.args.ctx); (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
			deleted, err := deleteSeries(tt.fields.matchName, tt.fields.matchValue)
			if err != nil {
				t.Fatalf("error delete series: %s", err)
			}
			if int64(deleted) != tt.numOfSeries {
				t.Fatalf("expected deleted series %d; got deleted series %d", tt.numOfSeries, deleted)
			}
		})
	}
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
	return vmstorage.DeleteSeries(nil, []*storage.TagFilters{tfs})
}

func Test_buildMatchWithFilter(t *testing.T) {
	tests := []struct {
		name       string
		filter     string
		metricName string
		want       string
		wantErr    bool
	}{
		{
			name:       "parsed metric with label",
			filter:     `{__name__="http_request_count_total",cluster="kube1"}`,
			metricName: "http_request_count_total",
			want:       `{cluster="kube1",__name__="http_request_count_total"}`,
			wantErr:    false,
		},
		{
			name:       "metric name with label",
			filter:     `http_request_count_total{cluster="kube1"}`,
			metricName: "http_request_count_total",
			want:       `{cluster="kube1",__name__="http_request_count_total"}`,
			wantErr:    false,
		},
		{
			name:       "parsed metric with regexp value",
			filter:     `{__name__="http_request_count_total",cluster=~"kube.*"}`,
			metricName: "http_request_count_total",
			want:       `{cluster=~"kube.*",__name__="http_request_count_total"}`,
			wantErr:    false,
		},
		{
			name:       "only label with regexp",
			filter:     `{cluster=~".*"}`,
			metricName: "http_request_count_total",
			want:       `{cluster=~".*",__name__="http_request_count_total"}`,
			wantErr:    false,
		},
		{
			name:       "many labels in filter with regexp",
			filter:     `{cluster=~".*",job!=""}`,
			metricName: "http_request_count_total",
			want:       `{cluster=~".*",job!="",__name__="http_request_count_total"}`,
			wantErr:    false,
		},
		{
			name:       "match with error",
			filter:     `{cluster~=".*"}`,
			metricName: "http_request_count_total",
			want:       ``,
			wantErr:    true,
		},
		{
			name:       "all names",
			filter:     `{__name__!=""}`,
			metricName: "http_request_count_total",
			want:       `{__name__="http_request_count_total"}`,
			wantErr:    false,
		},
		{
			name:       "with many underscores labels",
			filter:     `{__name__!="", __meta__!=""}`,
			metricName: "http_request_count_total",
			want:       `{__meta__!="",__name__="http_request_count_total"}`,
			wantErr:    false,
		},
		{
			name:       "metric name has regexp",
			filter:     `{__name__=~".*"}`,
			metricName: "http_request_count_total",
			want:       `{__name__="http_request_count_total"}`,
			wantErr:    false,
		},
		{
			name:       "metric name has negative regexp",
			filter:     `{__name__!~".*"}`,
			metricName: "http_request_count_total",
			want:       `{__name__="http_request_count_total"}`,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildMatchWithFilter(tt.filter, tt.metricName)
			if (err != nil) != tt.wantErr {
				t.Errorf("buildMatchWithFilter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("buildMatchWithFilter() got = %v, want %v", got, tt.want)
			}
		})
	}
}
