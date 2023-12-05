package main

import (
	"context"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/remotewrite"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/rule"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/templates"
)

func TestMain(m *testing.M) {
	if err := templates.Load([]string{"testdata/templates/*good.tmpl"}, true); err != nil {
		os.Exit(1)
	}
	os.Exit(m.Run())
}

// TestManagerEmptyRulesDir tests
// successful cases of
// starting with empty rules folder
func TestManagerEmptyRulesDir(t *testing.T) {
	m := &manager{groups: make(map[uint64]*rule.Group)}
	cfg := loadCfg(t, []string{"foo/bar"}, true, true)
	if err := m.update(context.Background(), cfg, false); err != nil {
		t.Fatalf("expected to load successfully with empty rules dir; got err instead: %v", err)
	}
}

// TestManagerUpdateConcurrent supposed to test concurrent
// execution of configuration update.
// Should be executed with -race flag
func TestManagerUpdateConcurrent(t *testing.T) {
	m := &manager{
		groups:         make(map[uint64]*rule.Group),
		querierBuilder: &datasource.FakeQuerier{},
		notifiers:      func() []notifier.Notifier { return []notifier.Notifier{&notifier.FakeNotifier{}} },
	}
	paths := []string{
		"config/testdata/dir/rules0-good.rules",
		"config/testdata/dir/rules0-bad.rules",
		"config/testdata/dir/rules1-good.rules",
		"config/testdata/dir/rules1-bad.rules",
		"config/testdata/rules/rules0-good.rules",
		"config/testdata/rules/rules1-good.rules",
		"config/testdata/rules/rules2-good.rules",
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
		go func(n int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(int64(n)))
			for i := 0; i < iterations; i++ {
				rnd := r.Intn(len(paths))
				cfg, err := config.Parse([]string{paths[rnd]}, notifier.ValidateTemplates, true)
				if err != nil { // update can fail and this is expected
					continue
				}
				_ = m.update(context.Background(), cfg, false)
			}
		}(i)
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
		VMRows = &rule.AlertingRule{
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
		Conns = &rule.AlertingRule{
			Name: "Conns",
			Expr: "sum(vm_tcplistener_conns) by(instance) > 1",
			Annotations: map[string]string{
				"summary":     "Too high connection number for {{$labels.instance}}",
				"description": "It is {{ $value }} connections for {{$labels.instance}}",
			},
		}
		ExampleAlertAlwaysFiring = &rule.AlertingRule{
			Name: "ExampleAlertAlwaysFiring",
			Expr: "sum by(job) (up == 1)",
		}
	)

	testCases := []struct {
		name       string
		initPath   string
		updatePath string
		want       []*rule.Group
	}{
		{
			name:       "update good rules",
			initPath:   "config/testdata/rules/rules0-good.rules",
			updatePath: "config/testdata/dir/rules1-good.rules",
			want: []*rule.Group{
				{
					File:     "config/testdata/dir/rules1-good.rules",
					Name:     "duplicatedGroupDiffFiles",
					Type:     config.NewPrometheusType(),
					Interval: defaultEvalInterval,
					Rules: []rule.Rule{
						&rule.AlertingRule{
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
			initPath:   "config/testdata/dir/rules/rules1-good.rules",
			updatePath: "config/testdata/rules/rules0-good.rules",
			want: []*rule.Group{
				{
					File:     "config/testdata/rules/rules0-good.rules",
					Name:     "groupGorSingleAlert",
					Type:     config.NewPrometheusType(),
					Interval: defaultEvalInterval,
					Rules:    []rule.Rule{VMRows},
				},
				{
					File:     "config/testdata/rules/rules0-good.rules",
					Interval: defaultEvalInterval,
					Type:     config.NewPrometheusType(),
					Name:     "TestGroup",
					Rules: []rule.Rule{
						Conns,
						ExampleAlertAlwaysFiring,
					},
				},
			},
		},
		{
			name:       "update with one bad rule file",
			initPath:   "config/testdata/rules/rules0-good.rules",
			updatePath: "config/testdata/dir/rules2-bad.rules",
			want: []*rule.Group{
				{
					File:     "config/testdata/rules/rules0-good.rules",
					Name:     "groupGorSingleAlert",
					Type:     config.NewPrometheusType(),
					Interval: defaultEvalInterval,
					Rules:    []rule.Rule{VMRows},
				},
				{
					File:     "config/testdata/rules/rules0-good.rules",
					Interval: defaultEvalInterval,
					Name:     "TestGroup",
					Type:     config.NewPrometheusType(),
					Rules: []rule.Rule{
						Conns,
						ExampleAlertAlwaysFiring,
					},
				},
			},
		},
		{
			name:       "update empty dir rules from 0 to 2 groups",
			initPath:   "config/testdata/empty/*",
			updatePath: "config/testdata/rules/rules0-good.rules",
			want: []*rule.Group{
				{
					File:     "config/testdata/rules/rules0-good.rules",
					Name:     "groupGorSingleAlert",
					Type:     config.NewPrometheusType(),
					Interval: defaultEvalInterval,
					Rules:    []rule.Rule{VMRows},
				},
				{
					File:     "config/testdata/rules/rules0-good.rules",
					Interval: defaultEvalInterval,
					Type:     config.NewPrometheusType(),
					Name:     "TestGroup",
					Rules: []rule.Rule{
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
				groups:         make(map[uint64]*rule.Group),
				querierBuilder: &datasource.FakeQuerier{},
				notifiers:      func() []notifier.Notifier { return []notifier.Notifier{&notifier.FakeNotifier{}} },
			}

			cfgInit := loadCfg(t, []string{tc.initPath}, true, true)
			if err := m.update(ctx, cfgInit, false); err != nil {
				t.Fatalf("failed to complete initial rules update: %s", err)
			}

			cfgUpdate, err := config.Parse([]string{tc.updatePath}, notifier.ValidateTemplates, true)
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
func compareGroups(t *testing.T, a, b *rule.Group) {
	t.Helper()
	if a.Name != b.Name {
		t.Fatalf("expected group name %q; got %q", a.Name, b.Name)
	}
	if a.File != b.File {
		t.Fatalf("expected group %q file name %q; got %q", a.Name, a.File, b.File)
	}
	if a.Interval != b.Interval {
		t.Fatalf("expected group %q interval %v; got %v", a.Name, a.Interval, b.Interval)
	}
	if len(a.Rules) != len(b.Rules) {
		t.Fatalf("expected group %s to have %d rules; got: %d",
			a.Name, len(a.Rules), len(b.Rules))
	}
	for i, r := range a.Rules {
		got, want := r, b.Rules[i]
		if a.ID() != b.ID() {
			t.Fatalf("expected to have rule %q; got %q", want.ID(), got.ID())
		}
		if err := rule.CompareRules(t, want, got); err != nil {
			t.Fatalf("comparison error: %s", err)
		}
	}
}

func TestManagerUpdateNegative(t *testing.T) {
	testCases := []struct {
		notifiers []notifier.Notifier
		rw        remotewrite.RWClient
		cfg       config.Group
		expErr    string
	}{
		{
			nil,
			nil,
			config.Group{
				Name: "Recording rule only",
				Rules: []config.Rule{
					{Record: "record", Expr: "max(up)"},
				},
			},
			"contains recording rules",
		},
		{
			nil,
			nil,
			config.Group{
				Name: "Alerting rule only",
				Rules: []config.Rule{
					{Alert: "alert", Expr: "up > 0"},
				},
			},
			"contains alerting rules",
		},
		{
			[]notifier.Notifier{&notifier.FakeNotifier{}},
			nil,
			config.Group{
				Name: "Recording and alerting rules",
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
			config.Group{
				Name: "Recording and alerting rules",
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
				groups:         make(map[uint64]*rule.Group),
				querierBuilder: &datasource.FakeQuerier{},
				rw:             tc.rw,
			}
			if tc.notifiers != nil {
				m.notifiers = func() []notifier.Notifier { return tc.notifiers }
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
	var validateTplFn config.ValidateTplFn
	if validateAnnotations {
		validateTplFn = notifier.ValidateTemplates
	}
	cfg, err := config.Parse(path, validateTplFn, validateExpressions)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
