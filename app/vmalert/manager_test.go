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

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

func TestMain(m *testing.M) {
	u, _ := url.Parse("https://victoriametrics.com/path")
	notifier.InitTemplateFunc(u)
	os.Exit(m.Run())
}

func TestManagerUpdateError(t *testing.T) {
	m := &manager{groups: make(map[uint64]*Group)}
	path := []string{"foo/bar"}
	err := m.update(context.Background(), path, true, true, false)
	if err == nil {
		t.Fatalf("expected to have err; got nil instead")
	}
	expErr := "no groups found"
	if !strings.Contains(err.Error(), expErr) {
		t.Fatalf("expected to got err %s; got %s", expErr, err)
	}
}

// TestManagerUpdateConcurrent supposed to test concurrent
// execution of configuration update.
// Should be executed with -race flag
func TestManagerUpdateConcurrent(t *testing.T) {
	m := &manager{
		groups:    make(map[uint64]*Group),
		querier:   &fakeQuerier{},
		notifiers: []notifier.Notifier{&fakeNotifier{}},
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
	*evaluationInterval = time.Millisecond
	if err := m.start(context.Background(), []string{paths[0]}, true, true); err != nil {
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
				path := []string{paths[rnd]}
				_ = m.update(context.Background(), path, true, true, false)
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
					Interval: defaultEvalInterval,
					Rules: []Rule{
						&AlertingRule{
							Name:   "VMRows",
							Expr:   "vm_rows > 0",
							For:    5 * time.Minute,
							Labels: map[string]string{"label": "bar"},
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
					Rules:    []Rule{VMRows},
					Interval: defaultEvalInterval,
				},
				{
					File:     "config/testdata/rules0-good.rules",
					Interval: defaultEvalInterval,
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
					Interval: defaultEvalInterval,
					Rules:    []Rule{VMRows},
				},
				{
					File:     "config/testdata/rules0-good.rules",
					Interval: defaultEvalInterval,
					Name:     "TestGroup", Rules: []Rule{
						Conns,
						ExampleAlertAlwaysFiring,
					}},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.TODO())
			m := &manager{groups: make(map[uint64]*Group), querier: &fakeQuerier{}}
			path := []string{tc.initPath}
			if err := m.update(ctx, path, true, true, false); err != nil {
				t.Fatalf("failed to complete initial rules update: %s", err)
			}

			path = []string{tc.updatePath}
			_ = m.update(ctx, path, true, true, false)
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
