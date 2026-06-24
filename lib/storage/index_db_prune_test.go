package storage

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/mergeset"
)

func TestPruneExpiredPerDayIndexItems(t *testing.T) {
	// minDate equals nowDate-2 for a 2-day retention, since 2*msecPerDay is a
	// multiple of msecPerDay. Every date < nowDate-2 is fully outside retention.
	nowDate := uint64(fasttime.UnixTimestamp()*1000) / msecPerDay

	// perDay builds a per-day index item: [nsPrefix][date][metricID].
	perDay := func(nsPrefix byte, date, metricID uint64) string {
		dst := marshalCommonPrefix(nil, nsPrefix)
		dst = encoding.MarshalUint64(dst, date)
		dst = encoding.MarshalUint64(dst, metricID)
		return string(dst)
	}
	// global builds a non per-day item that must never be pruned by date.
	global := func(nsPrefix byte, metricID uint64) string {
		dst := marshalCommonPrefix(nil, nsPrefix)
		dst = encoding.MarshalUint64(dst, metricID)
		return string(dst)
	}

	f := func(db *indexDB, items, expectedItems []string) {
		t.Helper()
		var data []byte
		var itemsB []mergeset.Item
		for _, item := range items {
			data = append(data, item...)
			itemsB = append(itemsB, mergeset.Item{
				Start: uint32(len(data) - len(item)),
				End:   uint32(len(data)),
			})
		}
		if !checkItemsSorted(data, itemsB) {
			t.Fatalf("source items aren't sorted; items:\n%v", itemsB)
		}
		resultData, resultItemsB := db.pruneExpiredPerDayIndexItems(data, itemsB)
		if !checkItemsSorted(resultData, resultItemsB) {
			t.Fatalf("result items aren't sorted; items:\n%v", resultItemsB)
		}
		var resultItems []string
		for _, it := range resultItemsB {
			resultItems = append(resultItems, string(it.Bytes(resultData)))
		}
		if len(resultItems) == 0 {
			resultItems = nil
		}
		if !reflect.DeepEqual(expectedItems, resultItems) {
			t.Fatalf("unexpected items;\ngot\n%X\nwant\n%X", resultItems, expectedItems)
		}
	}

	// 2-day retention.
	db := &indexDB{s: &Storage{retentionMsecs: 2 * msecPerDay}}

	expired := nowDate - 3 // fully outside retention
	retained := nowDate    // within retention

	// Expired per-day item in the middle is pruned for every per-day prefix.
	// A global item is placed first so the expired item is not at a boundary
	// (items must stay sorted, and within a per-day prefix they sort by date).
	for _, ns := range []byte{nsPrefixDateToMetricID, nsPrefixDateTagToMetricIDs, nsPrefixDateMetricNameToTSID} {
		f(db, []string{
			global(nsPrefixMetricIDToTSID, 1),
			perDay(ns, expired, 2),
			perDay(ns, retained, 3),
		}, []string{
			global(nsPrefixMetricIDToTSID, 1),
			perDay(ns, retained, 3),
		})
	}

	// First and last items must be preserved even when expired.
	f(db, []string{
		perDay(nsPrefixDateToMetricID, expired, 1),
		perDay(nsPrefixDateToMetricID, expired, 2),
		perDay(nsPrefixDateToMetricID, expired, 3),
	}, []string{
		perDay(nsPrefixDateToMetricID, expired, 1),
		perDay(nsPrefixDateToMetricID, expired, 3),
	})

	// Global (non per-day) items are never pruned by date.
	f(db, []string{
		global(nsPrefixMetricIDToTSID, 1),
		global(nsPrefixMetricIDToTSID, 2),
		global(nsPrefixMetricIDToTSID, 3),
	}, []string{
		global(nsPrefixMetricIDToTSID, 1),
		global(nsPrefixMetricIDToTSID, 2),
		global(nsPrefixMetricIDToTSID, 3),
	})

	// Mixed block: global items kept, expired per-day pruned, retained per-day kept.
	f(db, []string{
		global(nsPrefixMetricIDToTSID, 1),
		global(nsPrefixMetricIDToTSID, 2),
		perDay(nsPrefixDateToMetricID, expired, 1),
		perDay(nsPrefixDateToMetricID, retained, 2),
		perDay(nsPrefixDateMetricNameToTSID, expired, 3),
		perDay(nsPrefixDateMetricNameToTSID, retained, 4),
	}, []string{
		global(nsPrefixMetricIDToTSID, 1),
		global(nsPrefixMetricIDToTSID, 2),
		perDay(nsPrefixDateToMetricID, retained, 2),
		perDay(nsPrefixDateMetricNameToTSID, retained, 4),
	})

	// disablePerDayIndex: nothing is pruned.
	dbDisabled := &indexDB{s: &Storage{retentionMsecs: 2 * msecPerDay, disablePerDayIndex: true}}
	f(dbDisabled, []string{
		global(nsPrefixMetricIDToTSID, 1),
		perDay(nsPrefixDateToMetricID, expired, 2),
		perDay(nsPrefixDateToMetricID, retained, 3),
	}, []string{
		global(nsPrefixMetricIDToTSID, 1),
		perDay(nsPrefixDateToMetricID, expired, 2),
		perDay(nsPrefixDateToMetricID, retained, 3),
	})

	// Two-item blocks are returned unchanged (both are boundaries).
	f(db, []string{
		perDay(nsPrefixDateToMetricID, expired, 1),
		perDay(nsPrefixDateToMetricID, expired, 2),
	}, []string{
		perDay(nsPrefixDateToMetricID, expired, 1),
		perDay(nsPrefixDateToMetricID, expired, 2),
	})
}
