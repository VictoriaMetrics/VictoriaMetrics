package main

import (
	"context"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
)

func TestMain(m *testing.M) {
	u, _ := url.Parse("https://victoriametrics.com/path")
	notifier.InitTemplateFunc(u)
	os.Exit(m.Run())
}

// TestManagerEmptyRulesDir tests
// successful cases of
// starting with empty rules folder
func TestManagerEmptyRulesDir(t *testing.T) {
	m := &manager{groups: make(map[uint64]*Group)}
	cfg := loadCfg(t, []string{"foo/bar"}, true, true)
	if err := m.update(context.Background(), cfg, false); err != nil {
		t.Fatalf("expected to load succesfully with empty rules dir; got err instead: %v", err)
	}
}

// TestManagerUpdateConcurrent supposed to test concurrent
// execution of configuration update.
// Should be executed with -race flag
func TestManagerUpdateConcurrent(t *testing.T) {
	m := &manager{
		groups:         make(map[uint64]*Group),
		querierBuilder: &fakeQuerier{},
		notifiers:      []notifier.Notifier{&fakeNotifier{}},
	}
	paths := []string{
		"config/testdata/dir/rules0-good.rules",
		"config/testdata/dir/rules0-bad.rules",
		"config/testdata/dir/rules1-good.rules",
		"config/testdata/dir/rules1-bad.rules",
		"config/testdata/rules0-good.rules",
		"config/testdata/rules1-good.rules",
		"config/testdata/rules2-good.rules",
	}
	evalInterval := *evaluationInterval
	defer func() { *evaluationInterval = evalInterval }()
	*evaluationInterval = time.Millisecond
	cfg := loadCfg(t, []string{paths[0]}, true, true)
	if err := m.start(context.Background(), cfg); err != nil {
		t.Fatalf("failed to start: %s", err)
	}

	const workers = 500
	const iterations = 10
	wg := sync.WaitGroup{}
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				rnd := rand.Intn(len(paths))
				cfg, err := config.Parse([]string{paths[rnd]}, true, true)
				if err != nil { // update can fail and this is expected
					continue
				}
				_ = m.update(context.Background(), cfg, false)
			}
		}()
	}
	wg.Wait()
}

// TestManagerUpdate tests sequential configuration
// updates.
func TestManagerUpdate(t *testing.T) {
	const defaultEvalInterval = time.Second * 30
	currentEvalInterval := *evaluationInterval
	*evaluationInterval = defaultEvalInterval
	defer func() {
		*evaluationInterval = currentEvalInterval
	}()

	var (
		VMRows = &AlertingRule{
			Name: "VMRows",
			Expr: "vm_rows > 0",
			For:  10 * time.Second,
			Labels: map[string]string{
				"label": "bar",
				"host":  "{{ $labels.instance }}",
			},
			Annotations: map[string]string{
				"summary":     "{{ $value|humanize }}",
				"description": "{{$labels}}",
			},
		}
		Conns = &AlertingRule{
			Name: "Conns",
			Expr: "sum(vm_tcplistener_conns) by(instance) > 1",
			Annotations: map[string]string{
				"summary":     "Too high connection number for {{$labels.instance}}",
				"description": "It is {{ $value }} connections for {{$labels.instance}}",
			},
		}
		ExampleAlertAlwaysFiring = &AlertingRule{
			Name: "ExampleAlertAlwaysFiring",
			Expr: "sum by(job) (up == 1)",
		}
	)

	testCases := []struct {
		name       string
		initPath   string
		updatePath string
		want       []*Group
	}{
		{
			name:       "update good rules",
			initPath:   "config/testdata/rules0-good.rules",
			updatePath: "config/testdata/dir/rules1-good.rules",
			want: []*Group{
				{
					File:     "config/testdata/dir/rules1-good.rules",
					Name:     "duplicatedGroupDiffFiles",
					Type:     datasource.NewPrometheusType(),
					Interval: defaultEvalInterval,
					Rules: []Rule{
						&AlertingRule{
							Name:   "VMRows",
							Expr:   "vm_rows > 0",
							For:    5 * time.Minute,
							Labels: map[string]string{"dc": "gcp", "label": "bar"},
							Annotations: map[string]string{
								"summary":     "{{ $value }}",
								"description": "{{$labels}}",
							},
						},
					},
				},
			},
		},
		{
			name:       "update good rules from 1 to 2 groups",
			initPath:   "config/testdata/dir/rules1-good.rules",
			updatePath: "config/testdata/rules0-good.rules",
			want: []*Group{
				{
					File:     "config/testdata/rules0-good.rules",
					Name:     "groupGorSingleAlert",
					Type:     datasource.NewPrometheusType(),
					Rules:    []Rule{VMRows},
					Interval: defaultEvalInterval,
				},
				{
					File:     "config/testdata/rules0-good.rules",
					Interval: defaultEvalInterval,
					Type:     datasource.NewPrometheusType(),
					Name:     "TestGroup", Rules: []Rule{
						Conns,
						ExampleAlertAlwaysFiring,
					}},
			},
		},
		{
			name:       "update with one bad rule file",
			initPath:   "config/testdata/rules0-good.rules",
			updatePath: "config/testdata/dir/rules2-bad.rules",
			want: []*Group{
				{
					File:     "config/testdata/rules0-good.rules",
					Name:     "groupGorSingleAlert",
					Type:     datasource.NewPrometheusType(),
					Interval: defaultEvalInterval,
					Rules:    []Rule{VMRows},
				},
				{
					File:     "config/testdata/rules0-good.rules",
					Interval: defaultEvalInterval,
					Name:     "TestGroup",
					Type:     datasource.NewPrometheusType(),
					Rules: []Rule{
						Conns,
						ExampleAlertAlwaysFiring,
					}},
			},
		},
		{
			name:       "update empty dir rules from 0 to 2 groups",
			initPath:   "config/testdata/empty/*",
			updatePath: "config/testdata/rules0-good.rules",
			want: []*Group{
				{
					File:     "config/testdata/rules0-good.rules",
					Name:     "groupGorSingleAlert",
					Type:     datasource.NewPrometheusType(),
					Interval: defaultEvalInterval,
					Rules:    []Rule{VMRows},
				},
				{
					File:     "config/testdata/rules0-good.rules",
					Interval: defaultEvalInterval,
					Type:     datasource.NewPrometheusType(),
					Name:     "TestGroup", Rules: []Rule{
						Conns,
						ExampleAlertAlwaysFiring,
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.TODO())
			m := &manager{
				groups:         make(map[uint64]*Group),
				querierBuilder: &fakeQuerier{},
				notifiers:      []notifier.Notifier{&fakeNotifier{}},
			}

			cfgInit := loadCfg(t, []string{tc.initPath}, true, true)
			if err := m.update(ctx, cfgInit, false); err != nil {
				t.Fatalf("failed to complete initial rules update: %s", err)
			}

			cfgUpdate, err := config.Parse([]string{tc.updatePath}, true, true)
			if err == nil { // update can fail and that's expected
				_ = m.update(ctx, cfgUpdate, false)
			}
			if len(tc.want) != len(m.groups) {
				t.Fatalf("\nwant number of groups: %d;\ngot: %d ", len(tc.want), len(m.groups))
			}

			for _, wantG := range tc.want {
				gotG, ok := m.groups[wantG.ID()]
				if !ok {
					t.Fatalf("expected to have group %q", wantG.Name)
				}
				compareGroups(t, wantG, gotG)
			}

			cancel()
			m.close()
		})
	}
}

func TestManagerUpdateNegative(t *testing.T) {
	testCases := []struct {
		notifiers []notifier.Notifier
		rw        *remotewrite.Client
		cfg       config.Group
		expErr    string
	}{
		{
			nil,
			nil,
			config.Group{Name: "Recording rule only",
				Rules: []config.Rule{
					{Record: "record", Expr: "max(up)"},
				},
			},
			"contains recording rules",
		},
		{
			nil,
			nil,
			config.Group{Name: "Alerting rule only",
				Rules: []config.Rule{
					{Alert: "alert", Expr: "up > 0"},
				},
			},
			"contains alerting rules",
		},
		{
			[]notifier.Notifier{&fakeNotifier{}},
			nil,
			config.Group{Name: "Recording and alerting rules",
				Rules: []config.Rule{
					{Alert: "alert1", Expr: "up > 0"},
					{Alert: "alert2", Expr: "up > 0"},
					{Record: "record", Expr: "max(up)"},
				},
			},
			"contains recording rules",
		},
		{
			nil,
			&remotewrite.Client{},
			config.Group{Name: "Recording and alerting rules",
				Rules: []config.Rule{
					{Record: "record1", Expr: "max(up)"},
					{Record: "record2", Expr: "max(up)"},
					{Alert: "alert", Expr: "up > 0"},
				},
			},
			"contains alerting rules",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.cfg.Name, func(t *testing.T) {
			m := &manager{
				groups:         make(map[uint64]*Group),
				querierBuilder: &fakeQuerier{},
				notifiers:      tc.notifiers,
				rw:             tc.rw,
			}
			err := m.update(context.Background(), []config.Group{tc.cfg}, false)
			if err == nil {
				t.Fatalf("expected to get error; got nil")
			}
			if !strings.Contains(err.Error(), tc.expErr) {
				t.Fatalf("expected err to contain %q; got %q", tc.expErr, err)
			}
		})
	}
}

func loadCfg(t *testing.T, path []string, validateAnnotations, validateExpressions bool) []config.Group {
	t.Helper()
	cfg, err := config.Parse(path, validateAnnotations, validateExpressions)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
