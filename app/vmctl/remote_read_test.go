package main

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/prometheus/prompb"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/remoteread"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/testdata/servers_integration_test"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

func TestRemoteRead(t *testing.T) {
	barpool.Disable(true)
	defer func() {
		barpool.Disable(false)
	}()
	defer func() { isSilent = false }()

	var testCases = []struct {
		name             string
		remoteReadConfig remoteread.Config
		vmCfg            vm.Config
		start            string
		end              string
		numOfSamples     int64
		numOfSeries      int64
		rrp              remoteReadProcessor
		chunk            string
		remoteReadSeries func(start, end, numOfSeries, numOfSamples int64) []*prompb.TimeSeries
		expectedSeries   []vm.TimeSeries
	}{
		{
			name:             "step minute on minute time range",
			remoteReadConfig: remoteread.Config{Addr: "", LabelName: "__name__", LabelValue: ".*"},
			vmCfg:            vm.Config{Addr: "", Concurrency: 1},
			start:            "2022-11-26T11:23:05+02:00",
			end:              "2022-11-26T11:24:05+02:00",
			numOfSamples:     2,
			numOfSeries:      3,
			chunk:            stepper.StepMinute,
			remoteReadSeries: remote_read_integration.GenerateRemoteReadSeries,
			expectedSeries: []vm.TimeSeries{
				{
					Name:       "vm_metric_1",
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "0"}},
					Timestamps: []int64{1669454585000, 1669454615000},
					Values:     []float64{0, 0},
				},
				{
					Name:       "vm_metric_1",
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "1"}},
					Timestamps: []int64{1669454585000, 1669454615000},
					Values:     []float64{100, 100},
				},
				{
					Name:       "vm_metric_1",
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "2"}},
					Timestamps: []int64{1669454585000, 1669454615000},
					Values:     []float64{200, 200},
				},
			},
		},
		{
			name:             "step month on month time range",
			remoteReadConfig: remoteread.Config{Addr: "", LabelName: "__name__", LabelValue: ".*"},
			vmCfg: vm.Config{Addr: "", Concurrency: 1,
				Transport: http.DefaultTransport.(*http.Transport)},
			start:            "2022-09-26T11:23:05+02:00",
			end:              "2022-11-26T11:24:05+02:00",
			numOfSamples:     2,
			numOfSeries:      3,
			chunk:            stepper.StepMonth,
			remoteReadSeries: remote_read_integration.GenerateRemoteReadSeries,
			expectedSeries: []vm.TimeSeries{
				{
					Name:       "vm_metric_1",
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "0"}},
					Timestamps: []int64{1664184185000},
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
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "2"}},
					Timestamps: []int64{1664184185000},
					Values:     []float64{200},
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
					Timestamps: []int64{1666819415000},
					Values:     []float64{100},
				},
				{
					Name:       "vm_metric_1",
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "2"}},
					Timestamps: []int64{1666819415000},
					Values:     []float64{200}},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			remoteReadServer := remote_read_integration.NewRemoteReadServer(t)
			defer remoteReadServer.Close()
			remoteWriteServer := remote_read_integration.NewRemoteWriteServer(t)
			defer remoteWriteServer.Close()

			tt.remoteReadConfig.Addr = remoteReadServer.URL()

			rr, err := remoteread.NewClient(tt.remoteReadConfig)
			if err != nil {
				t.Fatalf("error create remote read client: %s", err)
			}

			start, err := time.Parse(time.RFC3339, tt.start)
			if err != nil {
				t.Fatalf("Error parse start time: %s", err)
			}

			end, err := time.Parse(time.RFC3339, tt.end)
			if err != nil {
				t.Fatalf("Error parse end time: %s", err)
			}

			rrs := tt.remoteReadSeries(start.Unix(), end.Unix(), tt.numOfSeries, tt.numOfSamples)

			remoteReadServer.SetRemoteReadSeries(rrs)
			remoteWriteServer.ExpectedSeries(tt.expectedSeries)

			tt.vmCfg.Addr = remoteWriteServer.URL()

			importer, err := vm.NewImporter(ctx, tt.vmCfg)
			if err != nil {
				t.Fatalf("failed to create VM importer: %s", err)
			}
			defer importer.Close()

			rmp := remoteReadProcessor{
				src: rr,
				dst: importer,
				filter: remoteReadFilter{
					timeStart: &start,
					timeEnd:   &end,
					chunk:     tt.chunk,
				},
				cc:        1,
				isVerbose: false,
			}

			err = rmp.run(ctx)
			if err != nil {
				t.Fatalf("failed to run remote read processor: %s", err)
			}
		})
	}
}

func TestSteamRemoteRead(t *testing.T) {
	barpool.Disable(true)
	defer func() {
		barpool.Disable(false)
	}()
	defer func() { isSilent = false }()

	var testCases = []struct {
		name             string
		remoteReadConfig remoteread.Config
		vmCfg            vm.Config
		start            string
		end              string
		numOfSamples     int64
		numOfSeries      int64
		rrp              remoteReadProcessor
		chunk            string
		remoteReadSeries func(start, end, numOfSeries, numOfSamples int64) []*prompb.TimeSeries
		expectedSeries   []vm.TimeSeries
	}{
		{
			name:             "step minute on minute time range",
			remoteReadConfig: remoteread.Config{Addr: "", LabelName: "__name__", LabelValue: ".*", UseStream: true},
			vmCfg:            vm.Config{Addr: "", Concurrency: 1},
			start:            "2022-11-26T11:23:05+02:00",
			end:              "2022-11-26T11:24:05+02:00",
			numOfSamples:     2,
			numOfSeries:      3,
			chunk:            stepper.StepMinute,
			remoteReadSeries: remote_read_integration.GenerateRemoteReadSeries,
			expectedSeries: []vm.TimeSeries{
				{
					Name:       "vm_metric_1",
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "0"}},
					Timestamps: []int64{1669454585000, 1669454615000},
					Values:     []float64{0, 0},
				},
				{
					Name:       "vm_metric_1",
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "1"}},
					Timestamps: []int64{1669454585000, 1669454615000},
					Values:     []float64{100, 100},
				},
				{
					Name:       "vm_metric_1",
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "2"}},
					Timestamps: []int64{1669454585000, 1669454615000},
					Values:     []float64{200, 200},
				},
			},
		},
		{
			name:             "step month on month time range",
			remoteReadConfig: remoteread.Config{Addr: "", LabelName: "__name__", LabelValue: ".*", UseStream: true},
			vmCfg:            vm.Config{Addr: "", Concurrency: 1},
			start:            "2022-09-26T11:23:05+02:00",
			end:              "2022-11-26T11:24:05+02:00",
			numOfSamples:     2,
			numOfSeries:      3,
			chunk:            stepper.StepMonth,
			remoteReadSeries: remote_read_integration.GenerateRemoteReadSeries,
			expectedSeries: []vm.TimeSeries{
				{
					Name:       "vm_metric_1",
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "0"}},
					Timestamps: []int64{1664184185000},
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
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "2"}},
					Timestamps: []int64{1664184185000},
					Values:     []float64{200},
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
					Timestamps: []int64{1666819415000},
					Values:     []float64{100},
				},
				{
					Name:       "vm_metric_1",
					LabelPairs: []vm.LabelPair{{Name: "job", Value: "2"}},
					Timestamps: []int64{1666819415000},
					Values:     []float64{200}},
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			remoteReadServer := remote_read_integration.NewRemoteReadStreamServer(t)
			defer remoteReadServer.Close()
			remoteWriteServer := remote_read_integration.NewRemoteWriteServer(t)
			defer remoteWriteServer.Close()

			tt.remoteReadConfig.Addr = remoteReadServer.URL()

			rr, err := remoteread.NewClient(tt.remoteReadConfig)
			if err != nil {
				t.Fatalf("error create remote read client: %s", err)
			}

			start, err := time.Parse(time.RFC3339, tt.start)
			if err != nil {
				t.Fatalf("Error parse start time: %s", err)
			}

			end, err := time.Parse(time.RFC3339, tt.end)
			if err != nil {
				t.Fatalf("Error parse end time: %s", err)
			}

			rrs := tt.remoteReadSeries(start.Unix(), end.Unix(), tt.numOfSeries, tt.numOfSamples)

			remoteReadServer.InitMockStorage(rrs)
			remoteWriteServer.ExpectedSeries(tt.expectedSeries)

			tt.vmCfg.Addr = remoteWriteServer.URL()

			importer, err := vm.NewImporter(ctx, tt.vmCfg)
			if err != nil {
				t.Fatalf("failed to create VM importer: %s", err)
			}
			defer importer.Close()

			rmp := remoteReadProcessor{
				src: rr,
				dst: importer,
				filter: remoteReadFilter{
					timeStart: &start,
					timeEnd:   &end,
					chunk:     tt.chunk,
				},
				cc:        1,
				isVerbose: false,
			}

			err = rmp.run(ctx)
			if err != nil {
				t.Fatalf("failed to run remote read processor: %s", err)
			}
		})
	}
}
