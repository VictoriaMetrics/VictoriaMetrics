package storage

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// withTSIDCacheKeyMode runs fn with the given key mode active and restores the
// previous mode afterwards. The mode is package-global, so tests relying on this
// must not call t.Parallel().
func withTSIDCacheKeyMode(t *testing.T, mode string, fn func()) {
	t.Helper()
	prevFlag := tsidCacheFingerprintKey
	SetTSIDCacheKeyMode(mode)
	defer func() {
		tsidCacheFingerprintKey = prevFlag
	}()
	fn()
}

// buildTestRows returns MetricRows for the given number of distinct series.
// The series share most of their labels and differ in the metric name and in a
// couple of label values, so that their marshaled metric names are mostly, but
// not fully, byte-identical.
func buildTestRows(series int) []MetricRow {
	ts := time.Now().UnixMilli()
	shared := []prompb.Label{
		{Name: "job", Value: "api"},
		{Name: "instance", Value: "host-01:9090"},
		{Name: "namespace", Value: "production"},
	}
	mrs := make([]MetricRow, 0, series)
	for i := 0; i < series; i++ {
		labels := append([]prompb.Label{
			{Name: "__name__", Value: fmt.Sprintf("request_duration_seconds_%04d", i/10)},
		}, shared...)
		labels = append(labels,
			prompb.Label{Name: "route", Value: fmt.Sprintf("/api/v%d", i/10)},
			prompb.Label{Name: "status_code", Value: fmt.Sprintf("%d", 200+i%10)},
		)
		mrs = append(mrs, MetricRow{
			MetricNameRaw: MarshalMetricNameRaw(nil, labels),
			Timestamp:     ts,
			Value:         float64(i),
		})
	}
	return mrs
}

// TestMarshalTSIDCacheKey pins the fingerprint key derivation to a fixed test
// vector. Any change to the derivation requires bumping tsidCacheFPFilename, so
// that cache files written by the previous derivation are not read back.
func TestMarshalTSIDCacheKey(t *testing.T) {
	raw := []byte("fixed test input")

	var kb [16]byte
	key, verifier := marshalTSIDCacheKey(&kb, raw)

	if got, want := hex.EncodeToString(key), "aed986b1d00ace49de381a574550dd76"; got != want {
		t.Fatalf("unexpected key: got %s, want %s", got, want)
	}
	if got, want := verifier, uint64(8792092503232340279); got != want {
		t.Fatalf("unexpected verifier: got %d, want %d", got, want)
	}
}

func TestTSIDCacheGetPut(t *testing.T) {
	for _, mode := range []string{tsidCacheKeyModeMetricName, tsidCacheKeyModeFingerprint} {
		t.Run(mode, func(t *testing.T) {
			withTSIDCacheKeyMode(t, mode, func() {
				s := MustOpenStorage(t.TempDir(), OpenOptions{})
				defer s.MustClose()

				mrs := buildTestRows(550)
				s.AddRows(mrs, 64)
				s.DebugFlush()

				// Every ingested series must be resolvable from the tsid cache
				// and must have a distinct MetricID.
				ids := make(map[uint64]int, len(mrs))
				var lTSID legacyTSID
				for i := range mrs {
					if !s.getTSIDByMetricNameFromCache(&lTSID, mrs[i].MetricNameRaw) {
						t.Fatalf("series %d not found in tsid cache after ingestion", i)
					}
					if lTSID.TSID.MetricID == 0 {
						t.Fatalf("series %d resolved to zero MetricID", i)
					}
					if prev, ok := ids[lTSID.TSID.MetricID]; ok {
						t.Fatalf("MetricID %d shared by series %d and %d", lTSID.TSID.MetricID, prev, i)
					}
					ids[lTSID.TSID.MetricID] = i
				}

				var m Metrics
				s.UpdateMetrics(&m)
				if m.TSIDCacheVerifierMisses != 0 {
					t.Fatalf("unexpected verifier mismatches: %d", m.TSIDCacheVerifierMisses)
				}
			})
		})
	}
}

func TestTSIDCacheVerifierMismatch(t *testing.T) {
	withTSIDCacheKeyMode(t, tsidCacheKeyModeFingerprint, func() {
		s := MustOpenStorage(t.TempDir(), OpenOptions{})
		defer s.MustClose()

		raw := MarshalMetricNameRaw(nil, []prompb.Label{
			{Name: "__name__", Value: "some_series"},
			{Name: "job", Value: "api"},
		})
		var kb [16]byte
		key, verifier := marshalTSIDCacheKey(&kb, raw)

		// An entry with the expected key but an unexpected verifier must be
		// rejected as a cache miss and counted.
		mismatching := fingerprintTSID{
			TSID:     TSID{MetricID: 123456},
			verifier: verifier + 1,
		}
		s.tsidCache.Set(key, fingerprintTSIDBytes(&mismatching))

		var lTSID legacyTSID
		if s.getTSIDByMetricNameFromCache(&lTSID, raw) {
			t.Fatalf("entry with an unexpected verifier must be rejected")
		}
		var m Metrics
		s.UpdateMetrics(&m)
		if m.TSIDCacheVerifierMisses != 1 {
			t.Fatalf("unexpected number of verifier mismatches: got %d, want 1", m.TSIDCacheVerifierMisses)
		}

		// An entry with the expected verifier must be accepted.
		matching := fingerprintTSID{
			TSID:     TSID{MetricID: 123456},
			verifier: verifier,
		}
		s.tsidCache.Set(key, fingerprintTSIDBytes(&matching))
		if !s.getTSIDByMetricNameFromCache(&lTSID, raw) {
			t.Fatalf("entry with the expected verifier must be accepted")
		}
		if lTSID.TSID.MetricID != 123456 {
			t.Fatalf("unexpected MetricID: got %d, want 123456", lTSID.TSID.MetricID)
		}
	})
}

func TestTSIDCacheFingerprintPersistence(t *testing.T) {
	withTSIDCacheKeyMode(t, tsidCacheKeyModeFingerprint, func() {
		dir := t.TempDir()
		mrs := buildTestRows(120)

		ids := make(map[string]uint64, len(mrs))
		s := MustOpenStorage(dir, OpenOptions{})
		s.AddRows(mrs, 64)
		s.DebugFlush()
		var lTSID legacyTSID
		for i := range mrs {
			if !s.getTSIDByMetricNameFromCache(&lTSID, mrs[i].MetricNameRaw) {
				t.Fatalf("series %d not found in tsid cache before restart", i)
			}
			ids[string(mrs[i].MetricNameRaw)] = lTSID.TSID.MetricID
		}
		s.MustClose()

		// The fingerprint cache must be persisted under its own filename.
		fpPath := filepath.Join(dir, cacheDirname, tsidCacheFPFilename)
		if !fs.IsPathExist(fpPath) {
			t.Fatalf("fingerprint tsid cache was not persisted at %s", fpPath)
		}
		mnPath := filepath.Join(dir, cacheDirname, tsidCacheFilename)
		if fs.IsPathExist(mnPath) {
			t.Fatalf("fingerprint mode must not write the metricName cache at %s", mnPath)
		}

		// Reopening must decode the persisted fingerprintTSID values.
		s = MustOpenStorage(dir, OpenOptions{})
		defer s.MustClose()
		for i := range mrs {
			if !s.getTSIDByMetricNameFromCache(&lTSID, mrs[i].MetricNameRaw) {
				t.Fatalf("series %d not found in tsid cache after restart", i)
			}
			if want := ids[string(mrs[i].MetricNameRaw)]; lTSID.TSID.MetricID != want {
				t.Fatalf("series %d has unexpected MetricID after restart: got %d, want %d",
					i, lTSID.TSID.MetricID, want)
			}
		}
		var m Metrics
		s.UpdateMetrics(&m)
		if m.TSIDCacheVerifierMisses != 0 {
			t.Fatalf("unexpected verifier mismatches after restart: %d", m.TSIDCacheVerifierMisses)
		}
	})
}

func TestTSIDCacheKeyModeSwitch(t *testing.T) {
	// Series identity comes from indexdb, so switching the key mode must not
	// change the TSIDs assigned to already registered series.
	transitions := []struct{ from, to string }{
		{tsidCacheKeyModeMetricName, tsidCacheKeyModeFingerprint},
		{tsidCacheKeyModeFingerprint, tsidCacheKeyModeMetricName},
	}
	for _, tr := range transitions {
		t.Run(tr.from+"_to_"+tr.to, func(t *testing.T) {
			dir := t.TempDir()
			mrs := buildTestRows(440)

			idsBefore := make(map[string]uint64, len(mrs))
			withTSIDCacheKeyMode(t, tr.from, func() {
				s := MustOpenStorage(dir, OpenOptions{})
				s.AddRows(mrs, 64)
				s.DebugFlush()
				var lTSID legacyTSID
				for i := range mrs {
					if !s.getTSIDByMetricNameFromCache(&lTSID, mrs[i].MetricNameRaw) {
						t.Fatalf("series %d missing before the mode switch", i)
					}
					idsBefore[string(mrs[i].MetricNameRaw)] = lTSID.TSID.MetricID
				}
				s.MustClose()
			})

			withTSIDCacheKeyMode(t, tr.to, func() {
				s := MustOpenStorage(dir, OpenOptions{})
				defer s.MustClose()

				var mBefore Metrics
				s.UpdateMetrics(&mBefore)

				// Re-ingesting the same series must not create new series.
				s.AddRows(mrs, 64)
				s.DebugFlush()

				var mAfter Metrics
				s.UpdateMetrics(&mAfter)
				if got := mAfter.NewTimeseriesCreated - mBefore.NewTimeseriesCreated; got != 0 {
					t.Fatalf("re-ingestion after the mode switch created %d new series; want 0", got)
				}

				var lTSID legacyTSID
				for i := range mrs {
					if !s.getTSIDByMetricNameFromCache(&lTSID, mrs[i].MetricNameRaw) {
						t.Fatalf("series %d missing after the mode switch", i)
					}
					if want := idsBefore[string(mrs[i].MetricNameRaw)]; lTSID.TSID.MetricID != want {
						t.Fatalf("series %d MetricID changed across the mode switch: got %d, want %d",
							i, lTSID.TSID.MetricID, want)
					}
				}
				if mAfter.TSIDCacheVerifierMisses != 0 {
					t.Fatalf("unexpected verifier mismatches after the mode switch: %d", mAfter.TSIDCacheVerifierMisses)
				}
			})
		})
	}
}
