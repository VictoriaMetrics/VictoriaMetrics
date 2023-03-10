package main

import (
	"context"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	remote_read_integration "github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/testdata/servers_integration_test"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

// If you want to run this test:
// 1. run two instances of victoriametrics and define -httpListenAddr for both or just for second instance
// 2. define srcAddr and dstAddr const with your victoriametrics addresses
// 3. define matchFilter const with your importing data
// 4. define timeStartFilter
// 5. run each test one by one

const (
	matchFilter     = `{job="avalanche"}`
	timeStartFilter = "2020-01-01T20:07:00Z"
	timeEndFilter   = "2020-08-01T20:07:00Z"
	srcAddr         = "http://127.0.0.1:8428"
	dstAddr         = "http://127.0.0.1:8528"
)

// This test simulates close process if user abort it
func Test_vmNativeProcessor_run(t *testing.T) {
	t.Skip()
	type fields struct {
		filter    native.Filter
		rateLimit int64
		dst       *native.Client
		src       *native.Client
	}
	tests := []struct {
		name    string
		fields  fields
		closer  func(cancelFunc context.CancelFunc)
		wantErr bool
	}{
		{
			name: "simulate syscall.SIGINT",
			fields: fields{
				filter: native.Filter{
					Match:     matchFilter,
					TimeStart: timeStartFilter,
				},
				rateLimit: 0,
				dst: &native.Client{
					Addr: dstAddr,
				},
				src: &native.Client{
					Addr: srcAddr,
				},
			},
			closer: func(cancelFunc context.CancelFunc) {
				time.Sleep(time.Second * 5)
				cancelFunc()
			},
			wantErr: true,
		},
		{
			name: "simulate correct work",
			fields: fields{
				filter: native.Filter{
					Match:     matchFilter,
					TimeStart: timeStartFilter,
				},
				rateLimit: 0,
				dst: &native.Client{
					Addr: dstAddr,
				},
				src: &native.Client{
					Addr: srcAddr,
				},
			},
			closer:  func(cancelFunc context.CancelFunc) {},
			wantErr: false,
		},
		{
			name: "simulate correct work with chunking",
			fields: fields{
				filter: native.Filter{
					Match:     matchFilter,
					TimeStart: timeStartFilter,
					TimeEnd:   timeEndFilter,
					Chunk:     stepper.StepMonth,
				},
				rateLimit: 0,
				dst: &native.Client{
					Addr: dstAddr,
				},
				src: &native.Client{
					Addr: srcAddr,
				},
			},
			closer:  func(cancelFunc context.CancelFunc) {},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancelFn := context.WithCancel(context.Background())
			p := &vmNativeProcessor{
				filter:    tt.fields.filter,
				rateLimit: tt.fields.rateLimit,
				dst:       tt.fields.dst,
				src:       tt.fields.src,
			}

			tt.closer(cancelFn)

			if err := p.run(ctx, true); (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_vmNativeProcessor_run1(t *testing.T) {
	type fields struct {
		filter       native.Filter
		dst          *native.Client
		src          *native.Client
		backoff      *backoff.Backoff
		s            *stats
		rateLimit    int64
		interCluster bool
		cc           int
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
			name:         "first test",
			start:        "2022-11-25T11:23:05+02:00",
			end:          "2022-11-27T11:24:05+02:00",
			numOfSamples: 2,
			numOfSeries:  3,
			chunk:        stepper.StepMinute,
			fields: fields{
				filter: native.Filter{
					Match: `{__name__!=""}`,
				},
				dst:          nil,
				src:          nil,
				backoff:      backoff.New(),
				rateLimit:    0,
				interCluster: false,
				cc:           1,
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

			tt.fields.filter.TimeStart = tt.start
			tt.fields.filter.TimeEnd = tt.end

			rws := tt.vmSeries(start.Unix(), end.Unix(), tt.numOfSeries, tt.numOfSamples)

			src.Series(rws)
			dst.ExpectedSeries(tt.expectedSeries)

			if err := src.InitFakeStorage(); err != nil {
				t.Fatalf("fail when trying to init fake storage: %s", err)
			}
			defer src.CloseStorage()

			tt.fields.src = &native.Client{
				AuthCfg:              nil,
				Addr:                 src.URL(),
				ExtraLabels:          []string{},
				DisableHTTPKeepAlive: false,
			}
			tt.fields.dst = &native.Client{
				AuthCfg:              nil,
				Addr:                 dst.URL(),
				ExtraLabels:          []string{},
				DisableHTTPKeepAlive: false,
			}

			p := &vmNativeProcessor{
				filter:       tt.fields.filter,
				dst:          tt.fields.dst,
				src:          tt.fields.src,
				backoff:      tt.fields.backoff,
				s:            tt.fields.s,
				rateLimit:    tt.fields.rateLimit,
				interCluster: tt.fields.interCluster,
				cc:           tt.fields.cc,
			}

			if err := p.run(tt.args.ctx, tt.args.silent); (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
