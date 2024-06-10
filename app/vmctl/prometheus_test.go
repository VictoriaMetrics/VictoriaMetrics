package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/vm"
)

// If you want to run this test:
// 1. provide test snapshot path in const testSnapshot
// 2. define httpAddr const with your victoriametrics address
// 3. run victoria metrics with defined address
// 4. remove t.Skip() from Test_prometheusProcessor_run
// 5. run tests one by one not all at one time

const (
	httpAddr     = "http://127.0.0.1:8428/"
	testSnapshot = "./testdata/20220427T130947Z-70ba49b1093fd0bf"
)

// This test simulates close process if user abort it
func Test_prometheusProcessor_run(t *testing.T) {
	t.Skip()

	defer func() { isSilent = false }()

	type fields struct {
		cfg    prometheus.Config
		vmCfg  vm.Config
		cl     func(prometheus.Config) *prometheus.Client
		im     func(vm.Config) *vm.Importer
		closer func(importer *vm.Importer)
		cc     int
	}
	type args struct {
		silent  bool
		verbose bool
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "simulate syscall.SIGINT",
			fields: fields{
				cfg: prometheus.Config{
					Snapshot: testSnapshot,
					Filter:   prometheus.Filter{},
				},
				cl: func(cfg prometheus.Config) *prometheus.Client {
					client, err := prometheus.NewClient(cfg)
					if err != nil {
						t.Fatalf("error init prometeus client: %s", err)
					}
					return client
				},
				im: func(vmCfg vm.Config) *vm.Importer {
					importer, err := vm.NewImporter(context.Background(), vmCfg)
					if err != nil {
						t.Fatalf("error init importer: %s", err)
					}
					return importer
				},
				closer: func(importer *vm.Importer) {
					// simulate syscall.SIGINT
					time.Sleep(time.Second * 5)
					if importer != nil {
						importer.Close()
					}
				},
				vmCfg: vm.Config{Addr: httpAddr, Concurrency: 1},
				cc:    2,
			},
			args: args{
				silent:  false,
				verbose: false,
			},
			wantErr: true,
		},
		{
			name: "simulate correct work",
			fields: fields{
				cfg: prometheus.Config{
					Snapshot: testSnapshot,
					Filter:   prometheus.Filter{},
				},
				cl: func(cfg prometheus.Config) *prometheus.Client {
					client, err := prometheus.NewClient(cfg)
					if err != nil {
						t.Fatalf("error init prometeus client: %s", err)
					}
					return client
				},
				im: func(vmCfg vm.Config) *vm.Importer {
					importer, err := vm.NewImporter(context.Background(), vmCfg)
					if err != nil {
						t.Fatalf("error init importer: %s", err)
					}
					return importer
				},
				closer: nil,
				vmCfg:  vm.Config{Addr: httpAddr, Concurrency: 5},
				cc:     2,
			},
			args: args{
				silent:  true,
				verbose: false,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := tt.fields.cl(tt.fields.cfg)
			importer := tt.fields.im(tt.fields.vmCfg)
			isSilent = tt.args.silent
			pp := &prometheusProcessor{
				cl:        client,
				im:        importer,
				cc:        tt.fields.cc,
				isVerbose: tt.args.verbose,
			}

			// we should answer on prompt
			if !tt.args.silent {
				input := []byte("Y\n")

				r, w, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}

				_, err = w.Write(input)
				if err != nil {
					t.Error(err)
				}
				err = w.Close()
				if err != nil {
					t.Error(err)
				}

				stdin := os.Stdin
				// Restore stdin right after the test.
				defer func() {
					os.Stdin = stdin
					_ = r.Close()
					_ = w.Close()
				}()
				os.Stdin = r
			}

			// simulate close if needed
			if tt.fields.closer != nil {
				go tt.fields.closer(importer)
			}

			if err := pp.run(); (err != nil) != tt.wantErr {
				t.Errorf("run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
