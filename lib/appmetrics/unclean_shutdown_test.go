package appmetrics

import (
	"bytes"
	"errors"
	stdos "os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestUncleanShutdownMarkerLifecycle(t *testing.T) {
	resetUncleanShutdownStateForTest(t)

	dirPath := filepath.Join(t.TempDir(), "data")
	markerPath := filepath.Join(dirPath, UncleanShutdownMarkerFilename)

	var bb bytes.Buffer
	writePrometheusMetrics(&bb)
	if strings.Contains(bb.String(), "vm_app_started_after_unclean_shutdown") {
		t.Fatalf("unexpected unclean shutdown metric before starting the marker")
	}

	MustStartUncleanShutdownMarker(dirPath)
	mustContainUncleanShutdownMetric(t, 0)
	if _, err := stdos.Stat(markerPath); err != nil {
		t.Fatalf("cannot stat the running marker after the first start: %s", err)
	}
	MustStopUncleanShutdownMarker()
	if _, err := stdos.Stat(markerPath); !stdos.IsNotExist(err) {
		t.Fatalf("unexpected running marker after a clean shutdown; got error %v; want os.ErrNotExist", err)
	}

	if err := stdos.WriteFile(markerPath, nil, 0600); err != nil {
		t.Fatalf("cannot create test marker: %s", err)
	}
	MustStartUncleanShutdownMarker(dirPath)
	mustContainUncleanShutdownMetric(t, 1)
	MustStopUncleanShutdownMarker()
	if _, err := stdos.Stat(markerPath); !stdos.IsNotExist(err) {
		t.Fatalf("unexpected running marker after a clean shutdown; got error %v; want os.ErrNotExist", err)
	}

	MustStartUncleanShutdownMarker(dirPath)
	mustContainUncleanShutdownMetric(t, 1)
	if err := stdos.Remove(markerPath); err != nil {
		t.Fatalf("cannot remove running marker: %s", err)
	}
	MustStopUncleanShutdownMarker()
}

func TestMustStartUncleanShutdownMarkerCalledTwicePanics(t *testing.T) {
	resetUncleanShutdownStateForTest(t)

	dirPath := t.TempDir()
	MustStartUncleanShutdownMarker(dirPath)
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expecting a panic")
		}
	}()
	MustStartUncleanShutdownMarker(dirPath)
}

func TestMustCreateUncleanShutdownMarkerConcurrent(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), UncleanShutdownMarkerFilename)

	const concurrency = 10
	createdCh := make(chan bool, concurrency)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			createdCh <- mustCreateUncleanShutdownMarker(markerPath)
		}()
	}
	wg.Wait()
	close(createdCh)

	created := 0
	for ok := range createdCh {
		if ok {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("unexpected number of successful marker creations; got %d; want 1", created)
	}
}

func TestRemoveUncleanShutdownMarkerRetries(t *testing.T) {
	errTest := errors.New("test remove error")
	removeCalls := 0
	sleepCalls := 0
	remove := func(_ string) error {
		removeCalls++
		if removeCalls < 3 {
			return errTest
		}
		return nil
	}
	sleep := func(d time.Duration) {
		sleepCalls++
		if d != uncleanShutdownMarkerRemoveRetry {
			t.Fatalf("unexpected retry delay; got %s; want %s", d, uncleanShutdownMarkerRemoveRetry)
		}
	}
	if err := removeUncleanShutdownMarker("test-marker", remove, sleep); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if removeCalls != 3 {
		t.Fatalf("unexpected number of remove calls; got %d; want 3", removeCalls)
	}
	if sleepCalls != 2 {
		t.Fatalf("unexpected number of sleep calls; got %d; want 2", sleepCalls)
	}
}

func TestRemoveUncleanShutdownMarkerStopsAfterLastAttempt(t *testing.T) {
	errTest := errors.New("test remove error")
	removeCalls := 0
	sleepCalls := 0
	remove := func(_ string) error {
		removeCalls++
		return errTest
	}
	sleep := func(_ time.Duration) {
		sleepCalls++
	}
	err := removeUncleanShutdownMarker("test-marker", remove, sleep)
	if !errors.Is(err, errTest) {
		t.Fatalf("unexpected error; got %v; want %v", err, errTest)
	}
	if removeCalls != uncleanShutdownMarkerRemoveAttempts {
		t.Fatalf("unexpected number of remove calls; got %d; want %d", removeCalls, uncleanShutdownMarkerRemoveAttempts)
	}
	if sleepCalls != uncleanShutdownMarkerRemoveAttempts-1 {
		t.Fatalf("unexpected number of sleep calls; got %d; want %d", sleepCalls, uncleanShutdownMarkerRemoveAttempts-1)
	}
}

func TestRemoveUncleanShutdownMarkerIgnoresNotExist(t *testing.T) {
	removeCalls := 0
	sleepCalls := 0
	remove := func(_ string) error {
		removeCalls++
		return stdos.ErrNotExist
	}
	sleep := func(_ time.Duration) {
		sleepCalls++
	}
	if err := removeUncleanShutdownMarker("test-marker", remove, sleep); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if removeCalls != 1 {
		t.Fatalf("unexpected number of remove calls; got %d; want 1", removeCalls)
	}
	if sleepCalls != 0 {
		t.Fatalf("unexpected number of sleep calls; got %d; want 0", sleepCalls)
	}
}

func resetUncleanShutdownStateForTest(t *testing.T) {
	reset := func() {
		uncleanShutdownMarkerDirPath = ""
		uncleanShutdownMetricEnabled.Store(false)
		startedAfterUncleanShutdown.Store(0)
	}
	reset()
	t.Cleanup(reset)
}

func mustContainUncleanShutdownMetric(t *testing.T, value uint64) {
	t.Helper()

	var bb bytes.Buffer
	writePrometheusMetrics(&bb)
	want := "vm_app_started_after_unclean_shutdown " + strconv.FormatUint(value, 10) + "\n"
	if !strings.Contains(bb.String(), want) {
		t.Fatalf("missing %q in the exported app metrics", want)
	}
}
