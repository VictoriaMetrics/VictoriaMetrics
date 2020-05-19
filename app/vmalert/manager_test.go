package main

import (
	"context"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestManagerUpdateError(t *testing.T) {
	m := &manager{groups: make(map[uint64]*Group)}
	path := []string{"foo/bar"}
	err := m.update(context.Background(), path, true, false)
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
		groups:   make(map[uint64]*Group),
		storage:  &fakeQuerier{},
		notifier: &fakeNotifier{},
	}
	paths := []string{
		"testdata/dir/rules0-good.rules",
		"testdata/dir/rules1-good.rules",
		"testdata/rules0-good.rules",
	}
	*evaluationInterval = time.Millisecond
	if err := m.start(context.Background(), []string{paths[0]}, true); err != nil {
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
				err := m.update(context.Background(), path, true, false)
				if err != nil {
					t.Errorf("update error: %s", err)
				}
			}
		}()
	}
	wg.Wait()
}

// TestManagerUpdate tests sequential configuration
// updates.
func TestManagerUpdate(t *testing.T) {
	testCases := []struct {
		name       string
		initPath   string
		updatePath string
		want       []*Group
	}{
		{
			name:       "update good rules",
			initPath:   "testdata/rules0-good.rules",
			updatePath: "testdata/dir/rules1-good.rules",
			want: []*Group{
				{
					File:  "testdata/dir/rules1-good.rules",
					Name:  "duplicatedGroupDiffFiles",
					Rules: []*Rule{newTestRule("VMRows", time.Second*10)},
				},
			},
		},
		{
			name:       "update good rules from 1 to 2 groups",
			initPath:   "testdata/dir/rules1-good.rules",
			updatePath: "testdata/rules0-good.rules",
			want: []*Group{
				{
					File: "testdata/rules0-good.rules",
					Name: "groupGorSingleAlert", Rules: []*Rule{
						newTestRule("VMRows", time.Second*10),
					}},
				{
					File: "testdata/rules0-good.rules",
					Name: "TestGroup", Rules: []*Rule{
						newTestRule("Conns", time.Duration(0)),
						newTestRule("ExampleAlertAlwaysFiring", time.Duration(0)),
					}},
			},
		},
		{
			name:       "update with one bad rule file",
			initPath:   "testdata/rules0-good.rules",
			updatePath: "testdata/dir/rules2-bad.rules",
			want: []*Group{
				{
					File: "testdata/rules0-good.rules",
					Name: "groupGorSingleAlert", Rules: []*Rule{
						newTestRule("VMRows", time.Second*10),
					}},
				{
					File: "testdata/rules0-good.rules",
					Name: "TestGroup", Rules: []*Rule{
						newTestRule("Conns", time.Duration(0)),
						newTestRule("ExampleAlertAlwaysFiring", time.Duration(0)),
					}},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.TODO())
			m := &manager{groups: make(map[uint64]*Group), storage: &fakeQuerier{}}
			path := []string{tc.initPath}
			if err := m.update(ctx, path, true, false); err != nil {
				t.Fatalf("failed to complete initial rules update: %s", err)
			}

			path = []string{tc.updatePath}
			_ = m.update(ctx, path, true, false)
			if len(tc.want) != len(m.groups) {
				t.Fatalf("\nwant number of groups: %d;\ngot: %d ", len(tc.want), len(m.groups))
			}

			for _, wantG := range tc.want {
				gotG, ok := m.groups[wantG.ID()]
				if !ok {
					t.Fatalf("expected to have group %q", wantG.Name)
				}
				compareGroups(t, gotG, wantG)
			}

			cancel()
			m.close()
		})
	}
}

func compareGroups(t *testing.T, a, b *Group) {
	t.Helper()
	if len(a.Rules) != len(b.Rules) {
		t.Fatalf("expected group %s to have %d rules; got: %d",
			a.Name, len(a.Rules), len(b.Rules))
	}
	for i, r := range a.Rules {
		got, want := r, b.Rules[i]
		if got.Name != want.Name {
			t.Fatalf("expected to have rule %q; got %q", want.Name, got.Name)
		}
	}
}
