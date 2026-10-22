package storage

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestApplyTimeRangeToSnapshot(t *testing.T) {
	defer testRemoveAll(t)

	path := t.Name()
	_ = os.RemoveAll(path)
	retention := 180 * 24 * time.Hour
	s := MustOpenStorage(path, OpenOptions{Retention: retention})
	defer func() {
		s.MustClose()
	}()

	now := time.Now().UTC()
	oldDay := time.Date(now.Year(), now.Month(), 15, 0, 0, 0, 0, time.UTC).AddDate(0, -2, 0)
	newDay := time.Date(now.Year(), now.Month(), 15, 0, 0, 0, 0, time.UTC)
	if !newDay.Before(now) {
		newDay = now.Add(-48 * time.Hour)
	}
	rng := rand.New(rand.NewSource(1))
	oldTR := TimeRange{
		MinTimestamp: oldDay.UnixMilli(),
		MaxTimestamp: oldDay.Add(23*time.Hour + 59*time.Minute).UnixMilli(),
	}
	newTR := TimeRange{
		MinTimestamp: newDay.UnixMilli(),
		MaxTimestamp: newDay.Add(23*time.Hour + 59*time.Minute).UnixMilli(),
	}
	oldRows := testGenerateMetricRowsWithPrefix(rng, 50, "old", oldTR)
	newRows := testGenerateMetricRowsWithPrefix(rng, 50, "new", newTR)
	s.AddRows(append(oldRows, newRows...), defaultPrecisionBits)
	s.DebugFlush()
	if err := s.ForceMergePartitions(""); err != nil {
		t.Fatalf("force merge live storage: %s", err)
	}

	snapshotName := s.MustCreateSnapshot()
	snapshotPath := filepath.Join(s.path, snapshotsDirname, snapshotName)
	snap := MustOpenStorage(snapshotPath, OpenOptions{Retention: retention, DisableBackgroundMerges: true})
	defer func() {
		snap.MustClose()
		RemoveStorageRuntimeFiles(snapshotPath)
	}()

	if err := snap.ApplyTimeRangeToSnapshot(context.Background(), newTR.MinTimestamp, newTR.MaxTimestamp, false); err != nil {
		t.Fatalf("ApplyTimeRangeToSnapshot() failed: %s", err)
	}

	ptws := snap.tb.GetAllPartitions(nil)
	keptParts := 0
	for _, ptw := range ptws {
		pws := ptw.pt.GetParts(nil, true)
		for _, pw := range pws {
			partTR := TimeRange{MinTimestamp: pw.p.ph.MinTimestamp, MaxTimestamp: pw.p.ph.MaxTimestamp}
			if partTR.MinTimestamp < newTR.MinTimestamp || partTR.MaxTimestamp >= newTR.MaxTimestamp {
				ptw.pt.PutParts(pws)
				snap.tb.PutPartitions(ptws)
				t.Fatalf("kept part %q timestamps [%d, %d] are not inside window [%d, %d)", pw.p.path, partTR.MinTimestamp, partTR.MaxTimestamp, newTR.MinTimestamp, newTR.MaxTimestamp)
			}
			keptParts++
		}
		ptw.pt.PutParts(pws)
	}
	snap.tb.PutPartitions(ptws)
	if keptParts == 0 {
		t.Fatal("expected overlapping parts to remain after window select")
	}

	var liveMetrics Metrics
	s.UpdateMetrics(&liveMetrics)
	if liveMetrics.TableMetrics.TotalRowsCount() < 100 {
		t.Fatalf("live storage lost rows after snapshot windowing; got %d", liveMetrics.TableMetrics.TotalRowsCount())
	}
}

func TestApplyTimeRangeToSnapshotTrimsSamples(t *testing.T) {
	defer testRemoveAll(t)

	path := t.Name()
	_ = os.RemoveAll(path)
	retention := 180 * 24 * time.Hour
	s := MustOpenStorage(path, OpenOptions{Retention: retention})
	defer s.MustClose()

	day1 := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	rng := rand.New(rand.NewSource(1))
	day1Rows := testGenerateMetricRowsWithPrefix(rng, 50, "day", TimeRange{
		MinTimestamp: day1.UnixMilli(),
		MaxTimestamp: day2.UnixMilli() - 1,
	})
	day2Rows := testGenerateMetricRowsWithPrefix(rng, 50, "day", TimeRange{
		MinTimestamp: day2.UnixMilli(),
		MaxTimestamp: day3.UnixMilli() - 1,
	})
	endSample := day2Rows[0]
	endSample.MetricNameRaw = append([]byte(nil), endSample.MetricNameRaw...)
	endSample.Timestamp = day3.UnixMilli()
	s.AddRows(append(append(day1Rows, day2Rows...), endSample), defaultPrecisionBits)
	s.DebugFlush()
	if err := s.ForceMergePartitions(""); err != nil {
		t.Fatalf("force merge live storage: %s", err)
	}

	spanning := false
	ptwsLive := s.tb.GetAllPartitions(nil)
	for _, ptw := range ptwsLive {
		pws := ptw.pt.GetParts(nil, true)
		for _, pw := range pws {
			if pw.p.ph.MinTimestamp < day2.UnixMilli() && pw.p.ph.MaxTimestamp >= day2.UnixMilli() {
				spanning = true
			}
		}
		ptw.pt.PutParts(pws)
	}
	s.tb.PutPartitions(ptwsLive)
	if !spanning {
		t.Fatal("expected a merged part spanning both days so trim is observable")
	}

	snapshotName := s.MustCreateSnapshot()
	snapshotPath := filepath.Join(s.path, snapshotsDirname, snapshotName)
	snap := MustOpenStorage(snapshotPath, OpenOptions{Retention: retention, DisableBackgroundMerges: true})
	defer func() {
		snap.MustClose()
		RemoveStorageRuntimeFiles(snapshotPath)
	}()

	if err := snap.ApplyTimeRangeToSnapshot(context.Background(), day2.UnixMilli(), day3.UnixMilli(), false); err != nil {
		t.Fatalf("ApplyTimeRangeToSnapshot() failed: %s", err)
	}

	var rows uint64
	ptws := snap.tb.GetAllPartitions(nil)
	for _, ptw := range ptws {
		pws := ptw.pt.GetParts(nil, true)
		for _, pw := range pws {
			if pw.p.ph.MinTimestamp < day2.UnixMilli() || pw.p.ph.MaxTimestamp >= day3.UnixMilli() {
				ptw.pt.PutParts(pws)
				snap.tb.PutPartitions(ptws)
				t.Fatalf("kept part %q timestamps [%d, %d] are not inside [%d, %d)", pw.p.path, pw.p.ph.MinTimestamp, pw.p.ph.MaxTimestamp, day2.UnixMilli(), day3.UnixMilli())
			}
			rows += pw.p.ph.RowsCount
		}
		ptw.pt.PutParts(pws)
	}
	snap.tb.PutPartitions(ptws)
	if rows != 50 {
		t.Fatalf("trimmed snapshot rows = %d; want 50 (day 2 only)", rows)
	}
}

func TestApplyTimeRangeToSnapshotEndOnly(t *testing.T) {
	defer testRemoveAll(t)

	path := t.Name()
	_ = os.RemoveAll(path)
	retention := 180 * 24 * time.Hour
	s := MustOpenStorage(path, OpenOptions{Retention: retention})
	defer s.MustClose()

	day1 := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	day3 := time.Date(2026, 9, 9, 0, 0, 0, 0, time.UTC)
	rng := rand.New(rand.NewSource(1))
	day1Rows := testGenerateMetricRowsWithPrefix(rng, 50, "day", TimeRange{
		MinTimestamp: day1.UnixMilli(),
		MaxTimestamp: day2.UnixMilli() - 1,
	})
	day2Rows := testGenerateMetricRowsWithPrefix(rng, 50, "day", TimeRange{
		MinTimestamp: day2.UnixMilli(),
		MaxTimestamp: day3.UnixMilli() - 1,
	})
	s.AddRows(append(day1Rows, day2Rows...), defaultPrecisionBits)
	s.DebugFlush()
	if err := s.ForceMergePartitions(""); err != nil {
		t.Fatalf("force merge live storage: %s", err)
	}

	snapshotName := s.MustCreateSnapshot()
	snapshotPath := filepath.Join(s.path, snapshotsDirname, snapshotName)
	snap := MustOpenStorage(snapshotPath, OpenOptions{Retention: retention, DisableBackgroundMerges: true})
	defer func() {
		snap.MustClose()
		RemoveStorageRuntimeFiles(snapshotPath)
	}()

	if err := snap.ApplyTimeRangeToSnapshot(context.Background(), 0, day2.UnixMilli(), false); err != nil {
		t.Fatalf("ApplyTimeRangeToSnapshot(0, day2) failed: %s", err)
	}

	var rows uint64
	ptws := snap.tb.GetAllPartitions(nil)
	for _, ptw := range ptws {
		pws := ptw.pt.GetParts(nil, true)
		for _, pw := range pws {
			if pw.p.ph.MinTimestamp < 0 || pw.p.ph.MaxTimestamp >= day2.UnixMilli() {
				ptw.pt.PutParts(pws)
				snap.tb.PutPartitions(ptws)
				t.Fatalf("kept part %q timestamps [%d, %d] are not inside [0, %d)", pw.p.path, pw.p.ph.MinTimestamp, pw.p.ph.MaxTimestamp, day2.UnixMilli())
			}
			rows += pw.p.ph.RowsCount
		}
		ptw.pt.PutParts(pws)
	}
	snap.tb.PutPartitions(ptws)
	if rows != 50 {
		t.Fatalf("end-only snapshot rows = %d; want 50 (day 1 only)", rows)
	}
}

func TestApplyTimeRangeToSnapshotStartAfterEnd(t *testing.T) {
	defer testRemoveAll(t)

	s := MustOpenStorage(t.Name(), OpenOptions{})
	defer s.MustClose()
	if err := s.ApplyTimeRangeToSnapshot(context.Background(), 2000, 1000, false); err == nil {
		t.Fatal("expected error when start > end")
	}
	if err := s.ApplyTimeRangeToSnapshot(context.Background(), 1000, 1000, false); err == nil {
		t.Fatal("expected error when start == end")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.ApplyTimeRangeToSnapshot(ctx, 1000, 2000, false); err == nil {
		t.Fatal("expected error when context is canceled")
	}
}

func TestForceMergeAllPartsFailsWhenPartsUnclaimable(t *testing.T) {
	defer testRemoveAll(t)

	path := t.Name()
	_ = os.RemoveAll(path)
	s := MustOpenStorage(path, OpenOptions{})
	defer s.MustClose()

	now := time.Now().UTC()
	tr := TimeRange{
		MinTimestamp: now.Add(-time.Hour).UnixMilli(),
		MaxTimestamp: now.UnixMilli(),
	}
	rng := rand.New(rand.NewSource(1))
	s.AddRows(testGenerateMetricRowsWithPrefix(rng, 20, "unclaimable", tr), defaultPrecisionBits)
	s.DebugFlush()
	if err := s.ForceMergePartitions(""); err != nil {
		t.Fatalf("force merge live storage: %s", err)
	}

	ptws := s.tb.GetAllPartitions(nil)
	defer s.tb.PutPartitions(ptws)
	if len(ptws) == 0 {
		t.Fatal("expected at least one partition")
	}
	pt := ptws[0].pt
	if !pt.hasAnyParts() {
		t.Fatal("expected parts in partition")
	}

	setPartsInMerge(pt, true)
	defer setPartsInMerge(pt, false)

	stopCh := make(chan struct{})
	close(stopCh)
	err := pt.forceMergeAllParts(stopCh, true)
	if err == nil {
		t.Fatal("strict force merge must fail when parts exist but cannot be claimed")
	}
	if !strings.Contains(err.Error(), "cannot claim parts") {
		t.Fatalf("unexpected error: %s", err)
	}
}

func setPartsInMerge(pt *partition, inMerge bool) {
	pt.partsLock.Lock()
	defer pt.partsLock.Unlock()
	for _, pws := range [][]*partWrapper{pt.inmemoryParts, pt.smallParts, pt.bigParts} {
		for _, pw := range pws {
			pw.isInMerge = inMerge
		}
	}
}

func TestApplyTimeRangeFullWindowSkipsDropKeepsPreEpochParts(t *testing.T) {
	defer testRemoveAll(t)

	path := t.Name()
	_ = os.RemoveAll(path)
	s := MustOpenStorage(path, OpenOptions{DisableBackgroundMerges: true})
	defer s.MustClose()

	// Insert rejects negative timestamps, so attach a part header the way an
	// on-disk pre-epoch part would appear to the drop pass.
	preEpoch := &partWrapper{
		p: &part{
			ph: partHeader{MinTimestamp: -2000, MaxTimestamp: -1, RowsCount: 3},
		},
	}
	defer func() {
		ptws := s.tb.GetAllPartitions(nil)
		defer s.tb.PutPartitions(ptws)
		for _, ptw := range ptws {
			ptw.pt.partsLock.Lock()
			var kept []*partWrapper
			for _, pw := range ptw.pt.smallParts {
				if pw != preEpoch {
					kept = append(kept, pw)
				}
			}
			ptw.pt.smallParts = kept
			ptw.pt.partsLock.Unlock()
		}
	}()
	ptws := s.tb.GetAllPartitions(nil)
	if len(ptws) == 0 {
		s.tb.PutPartitions(ptws)
		t.Fatal("expected the current partition")
	}
	pt := ptws[0].pt
	pt.partsLock.Lock()
	pt.smallParts = append(pt.smallParts, preEpoch)
	pt.partsLock.Unlock()
	s.tb.PutPartitions(ptws)

	// forceMerge is false: this fake part has no files, so it cannot be compacted.
	// The assertion is that the full-range window returns before the drop pass.
	if err := s.ApplyTimeRangeToSnapshot(context.Background(), ForceMergeOnlyWindowStartMsec, ForceMergeOnlyWindowEndMsec, false); err != nil {
		t.Fatalf("full-range window failed: %s", err)
	}

	ptws = s.tb.GetAllPartitions(nil)
	defer s.tb.PutPartitions(ptws)
	found := false
	for _, ptw := range ptws {
		ptw.pt.partsLock.Lock()
		for _, pw := range ptw.pt.smallParts {
			if pw == preEpoch {
				found = true
			}
		}
		ptw.pt.partsLock.Unlock()
	}
	if !found {
		t.Fatal("full-range window dropped a part entirely before the Unix epoch")
	}
}

func TestDisableBackgroundMergesDoesNotStartPartMergers(t *testing.T) {
	defer testRemoveAll(t)

	root := t.Name()
	_ = os.RemoveAll(root)
	retention := 180 * 24 * time.Hour
	now := time.Now().UTC()
	tr := TimeRange{
		MinTimestamp: now.Add(-time.Hour).UnixMilli(),
		MaxTimestamp: now.UnixMilli(),
	}
	rng := rand.New(rand.NewSource(1))
	rows := testGenerateMetricRowsWithPrefix(rng, 10, "merge", tr)

	disabled := MustOpenStorage(filepath.Join(root, "disabled"), OpenOptions{
		Retention:               retention,
		DisableBackgroundMerges: true,
	})
	defer disabled.MustClose()
	disabled.AddRows(rows, defaultPrecisionBits)
	disabled.DebugFlush()
	if got := disabled.backgroundMergesStarted.Load(); got != 0 {
		t.Fatalf("DisableBackgroundMerges started %d part mergers", got)
	}

	live := MustOpenStorage(filepath.Join(root, "live"), OpenOptions{Retention: retention})
	defer live.MustClose()
	live.AddRows(rows, defaultPrecisionBits)
	live.DebugFlush()
	if got := live.backgroundMergesStarted.Load(); got == 0 {
		t.Fatal("expected live storage to start background part mergers")
	}
}

func TestDropExcludesPartThatOnlyTouchesExclusiveEnd(t *testing.T) {
	defer testRemoveAll(t)

	path := t.Name()
	_ = os.RemoveAll(path)
	retention := 180 * 24 * time.Hour
	s := MustOpenStorage(path, OpenOptions{Retention: retention})
	defer s.MustClose()

	end := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC).UnixMilli()
	start := end - int64(time.Hour/time.Millisecond)
	var mn MetricName
	mn.MetricGroup = []byte("boundary")
	mn.Tags = []Tag{
		{[]byte("job"), []byte("webservice")},
	}
	s.AddRows([]MetricRow{{
		MetricNameRaw: mn.marshalRaw(nil),
		Timestamp:     end,
		Value:         1,
	}}, defaultPrecisionBits)
	s.DebugFlush()
	if err := s.ForceMergePartitions(""); err != nil {
		t.Fatalf("force merge live storage: %s", err)
	}

	ptws := s.tb.GetAllPartitions(nil)
	parts := 0
	for _, ptw := range ptws {
		pws := ptw.pt.GetParts(nil, true)
		for _, pw := range pws {
			parts++
			if pw.p.ph.MinTimestamp != end {
				ptw.pt.PutParts(pws)
				s.tb.PutPartitions(ptws)
				t.Fatalf("part min = %d; want exclusive end %d", pw.p.ph.MinTimestamp, end)
			}
		}
		ptw.pt.PutParts(pws)
	}
	s.tb.PutPartitions(ptws)
	if parts == 0 {
		t.Fatal("expected a part whose min timestamp equals the exclusive end")
	}

	snapshotName := s.MustCreateSnapshot()
	snapshotPath := filepath.Join(s.path, snapshotsDirname, snapshotName)
	snap := MustOpenStorage(snapshotPath, OpenOptions{Retention: retention, DisableBackgroundMerges: true})
	defer func() {
		snap.MustClose()
		RemoveStorageRuntimeFiles(snapshotPath)
	}()

	err := snap.ApplyTimeRangeToSnapshot(context.Background(), start, end, false)
	if err == nil || !strings.Contains(err.Error(), "no parts overlap") {
		t.Fatalf("ApplyTimeRangeToSnapshot() error = %v; want no parts overlap", err)
	}
	if rows := snap.snapshotDataRows(); rows == 0 {
		t.Fatal("no-overlap window removed snapshot parts before refusing the backup")
	}
}

func TestApplyTimeRangeForceMergeOnlyKeepsSamplesOutsideRetention(t *testing.T) {
	defer testRemoveAll(t)

	path := t.Name()
	_ = os.RemoveAll(path)
	longRetention := 180 * 24 * time.Hour
	s := MustOpenStorage(path, OpenOptions{Retention: longRetention})
	defer s.MustClose()

	now := time.Now().UTC()
	oldTR := TimeRange{
		MinTimestamp: now.Add(-100 * 24 * time.Hour).UnixMilli(),
		MaxTimestamp: now.Add(-100*24*time.Hour + time.Hour).UnixMilli(),
	}
	newTR := TimeRange{
		MinTimestamp: now.Add(-30 * time.Minute).UnixMilli(),
		MaxTimestamp: now.Add(-time.Minute).UnixMilli(),
	}
	rng := rand.New(rand.NewSource(1))
	oldRows := testGenerateMetricRowsWithPrefix(rng, 20, "old", oldTR)
	newRows := testGenerateMetricRowsWithPrefix(rng, 20, "new", newTR)
	s.AddRows(append(oldRows, newRows...), defaultPrecisionBits)
	s.DebugFlush()
	if err := s.ForceMergePartitions(""); err != nil {
		t.Fatalf("force merge live storage: %s", err)
	}

	snapshotName := s.MustCreateSnapshot()
	snapshotPath := filepath.Join(s.path, snapshotsDirname, snapshotName)
	// Short retention would delete the 100-day samples during compaction
	// unless force-merge-only installs a keep-all trim range.
	snap := MustOpenStorage(snapshotPath, OpenOptions{
		Retention:               time.Hour,
		DisableBackgroundMerges: true,
	})
	defer func() {
		snap.MustClose()
		RemoveStorageRuntimeFiles(snapshotPath)
	}()

	if err := snap.ApplyTimeRangeToSnapshot(context.Background(), ForceMergeOnlyWindowStartMsec, ForceMergeOnlyWindowEndMsec, true); err != nil {
		t.Fatalf("force-merge-only failed: %s", err)
	}
	if rows := snap.snapshotDataRows(); rows != 40 {
		t.Fatalf("rows after force-merge-only = %d; want 40 (samples older than retention must stay)", rows)
	}
}
