package main

import (
	"context"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/native"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/stepper"
	remote_read_integration "github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/testdata/servers_integration_test"
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
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "first test",
			fields: fields{
				filter: native.Filter{
					Match:     `{__name__!=""}`,
					TimeStart: "2023-03-07T00:00:00Z",
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
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcStoragePath := "TestStorageSrc"
			dstStoragePath := "TestStorageDst"
			src := remote_read_integration.NewRemoteWriteServer(t)
			dst := remote_read_integration.NewRemoteWriteServer(t)
			src.InitStorage(srcStoragePath)
			dst.InitStorage(dstStoragePath)
			defer func() {
				src.CloseStorage(srcStoragePath)
				src.Close()
			}()
			defer func() {
				dst.CloseStorage(dstStoragePath)
				dst.Close()
			}()

			tt.fields.src = native.New(src.URL(), []string{}, nil)
			tt.fields.dst = native.New(dst.URL(), []string{}, nil)

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
