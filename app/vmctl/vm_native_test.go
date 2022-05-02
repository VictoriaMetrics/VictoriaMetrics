package main

import (
	"testing"
	"time"
)

// If you want to run this test:
// 1. run two instances of victoriametrics and define -httpListenAddr for both or just for second instance
// 2. define srcAddr and dstAddr const with your victoriametrics addresses
// 3. define matchFilter const with your importing data
// 4. define timeStartFilter

const (
	matchFilter     = `{job="avalanche"}`
	timeStartFilter = "2020-01-01T20:07:00Z"
	srcAddr         = "http://127.0.0.1:8428"
	dstAddr         = "http://127.0.0.1:8528"
)

// This test simulates close process if user abort it
func Test_vmNativeProcessor_run(t *testing.T) {
	t.Skip()
	type fields struct {
		filter    filter
		rateLimit int64
		dst       *vmNativeClient
		src       *vmNativeClient
	}
	tests := []struct {
		name    string
		fields  fields
		closer  func(shouldStop chan struct{})
		wantErr bool
	}{
		{
			name: "simulate syscall.SIGINT",
			fields: fields{
				filter: filter{
					match:     matchFilter,
					timeStart: timeStartFilter,
				},
				rateLimit: 0,
				dst: &vmNativeClient{
					addr: dstAddr,
				},
				src: &vmNativeClient{
					addr: srcAddr,
				},
			},
			closer: func(shouldStop chan struct{}) {
				time.Sleep(time.Second * 1)
				close(shouldStop)
			},
			wantErr: true,
		},
		{
			name: "simulate correct work",
			fields: fields{
				filter: filter{
					match:     matchFilter,
					timeStart: timeStartFilter,
				},
				rateLimit: 0,
				dst: &vmNativeClient{
					addr: dstAddr,
				},
				src: &vmNativeClient{
					addr: srcAddr,
				},
			},
			closer:  nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldStop := make(chan struct{})

			p := &vmNativeProcessor{
				filter:     tt.fields.filter,
				rateLimit:  tt.fields.rateLimit,
				dst:        tt.fields.dst,
				src:        tt.fields.src,
				shouldStop: shouldStop,
			}

			if tt.closer != nil {
				go tt.closer(shouldStop)
			}

			if err := p.run(); (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
