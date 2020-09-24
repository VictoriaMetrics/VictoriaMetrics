package storage

import (
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
)

func TestPartSearchOneRow(t *testing.T) {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = defaultPrecisionBits
	r.TSID.MetricID = 1234
	r.Timestamp = 100
	r.Value = 345
	rows = append(rows, r)

	p := newTestPart(rows)

	t.Run("EmptyTSID", func(t *testing.T) {
		tsids1 := []TSID{}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -100,
				MaxTimestamp: 3000,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})
	})

	t.Run("LowerTSID", func(t *testing.T) {
		tsids1 := []TSID{{MetricID: 10}}
		tsids2 := []TSID{{MetricID: 10}, {MetricID: 20}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -1000,
				MaxTimestamp: 300,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})
	})

	t.Run("HigherTSID", func(t *testing.T) {
		tsids1 := []TSID{{MetricID: 12345}}
		tsids2 := []TSID{{MetricID: 12345}, {MetricID: 12346}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -1000,
				MaxTimestamp: 300,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})
	})

	t.Run("LowerAndHihgerTSID", func(t *testing.T) {
		tsids1 := []TSID{{MetricID: 10}, {MetricID: 12345}}
		tsids2 := []TSID{{MetricID: 10}, {MetricID: 20}, {MetricID: 12345}, {MetricID: 12346}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -1000,
				MaxTimestamp: 300,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})
	})

	t.Run("MatchingOneTSID", func(t *testing.T) {
		rbs := []rawBlock{{
			TSID: TSID{
				MetricID: 1234,
			},
			Timestamps: []int64{100},
			Values:     []float64{345},
		}}

		tsids1 := []TSID{{MetricID: 1234}}
		tsids2 := []TSID{{MetricID: 1234}, {MetricID: 12345}}
		tsids3 := []TSID{{MetricID: 10}, {MetricID: 1234}}
		tsids4 := []TSID{{MetricID: 10}, {MetricID: 1234}, {MetricID: 12345}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -1000,
				MaxTimestamp: 300,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("InvalidTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: -2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})
	})

	t.Run("MatchingMultiTSID", func(t *testing.T) {
		// Results for duplicate tsids must be skipped.
		rbs := []rawBlock{{
			TSID: TSID{
				MetricID: 1234,
			},
			Timestamps: []int64{100},
			Values:     []float64{345},
		}}

		tsids1 := []TSID{{MetricID: 1234}, {MetricID: 1234}}
		tsids2 := []TSID{{MetricID: 1234}, {MetricID: 1234}, {MetricID: 12345}}
		tsids3 := []TSID{{MetricID: 10}, {MetricID: 1234}, {MetricID: 1234}}
		tsids4 := []TSID{{MetricID: 10}, {MetricID: 1234}, {MetricID: 1234}, {MetricID: 1234}, {MetricID: 12345}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -1000,
				MaxTimestamp: 300,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})
	})
}

func TestPartSearchTwoRowsOneTSID(t *testing.T) {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = defaultPrecisionBits
	r.TSID.MetricID = 1234

	r.Timestamp = 100
	r.Value = 345
	rows = append(rows, r)

	r.Timestamp = 200
	r.Value = 456
	rows = append(rows, r)

	p := newTestPart(rows)

	t.Run("EmptyTSID", func(t *testing.T) {
		tsids1 := []TSID{}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 10,
				MaxTimestamp: 300,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})
	})

	t.Run("LowerTSID", func(t *testing.T) {
		tsids1 := []TSID{{MetricID: 10}}
		tsids2 := []TSID{{MetricID: 10}, {MetricID: 20}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 10,
				MaxTimestamp: 300,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})
	})

	t.Run("HigherTSID", func(t *testing.T) {
		tsids1 := []TSID{{MetricID: 12345}}
		tsids2 := []TSID{{MetricID: 12345}, {MetricID: 12346}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 10,
				MaxTimestamp: 300,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})
	})

	t.Run("LowerAndHihgerTSID", func(t *testing.T) {
		tsids1 := []TSID{{MetricID: 10}, {MetricID: 12345}}
		tsids2 := []TSID{{MetricID: 10}, {MetricID: 20}, {MetricID: 12345}, {MetricID: 12346}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 10,
				MaxTimestamp: 300,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})
	})

	t.Run("MatchingOneTSID", func(t *testing.T) {
		rbs := []rawBlock{{
			TSID: TSID{
				MetricID: 1234,
			},
			Timestamps: []int64{100, 200},
			Values:     []float64{345, 456},
		}}

		tsids1 := []TSID{{MetricID: 1234}}
		tsids2 := []TSID{{MetricID: 1234}, {MetricID: 12345}}
		tsids3 := []TSID{{MetricID: 10}, {MetricID: 1234}}
		tsids4 := []TSID{{MetricID: 10}, {MetricID: 1234}, {MetricID: 12345}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -100,
				MaxTimestamp: 2000,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 200,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("LowerEndTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			rbs := []rawBlock{{
				TSID: TSID{
					MetricID: 1234,
				},
				Timestamps: []int64{100},
				Values:     []float64{345},
			}}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("LowerIntersectTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 60,
				MaxTimestamp: 150,
			}
			rbs := []rawBlock{{
				TSID: TSID{
					MetricID: 1234,
				},
				Timestamps: []int64{100},
				Values:     []float64{345},
			}}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("HigherEndTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 200,
				MaxTimestamp: 200,
			}
			rbs := []rawBlock{{
				TSID: TSID{
					MetricID: 1234,
				},
				Timestamps: []int64{200},
				Values:     []float64{456},
			}}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("HigherIntersectTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 150,
				MaxTimestamp: 240,
			}
			rbs := []rawBlock{{
				TSID: TSID{
					MetricID: 1234,
				},
				Timestamps: []int64{200},
				Values:     []float64{456},
			}}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("IvalidTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 200,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("InnerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 101,
				MaxTimestamp: 199,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})
	})
}

func TestPartSearchTwoRowsTwoTSID(t *testing.T) {
	var rows []rawRow
	var r rawRow
	r.PrecisionBits = defaultPrecisionBits

	r.TSID.MetricID = 1234
	r.Timestamp = 100
	r.Value = 345
	rows = append(rows, r)

	r.TSID.MetricID = 2345
	r.Timestamp = 200
	r.Value = 456
	rows = append(rows, r)

	p := newTestPart(rows)

	t.Run("EmptyTSID", func(t *testing.T) {
		tsids1 := []TSID{}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -100,
				MaxTimestamp: 1000,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
		})
	})

	t.Run("LowerTSID", func(t *testing.T) {
		tsids1 := []TSID{{MetricID: 10}}
		tsids2 := []TSID{{MetricID: 10}, {MetricID: 20}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -100,
				MaxTimestamp: 1000,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})
	})

	t.Run("HigherTSID", func(t *testing.T) {
		tsids1 := []TSID{{MetricID: 12345}}
		tsids2 := []TSID{{MetricID: 12345}, {MetricID: 12346}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -100,
				MaxTimestamp: 1000,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})
	})

	t.Run("LowerAndHihgerTSID", func(t *testing.T) {
		tsids1 := []TSID{{MetricID: 10}, {MetricID: 12345}}
		tsids2 := []TSID{{MetricID: 10}, {MetricID: 20}, {MetricID: 12345}, {MetricID: 12346}}
		tsids3 := []TSID{{MetricID: 10}, {MetricID: 1235}, {MetricID: 12345}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -100,
				MaxTimestamp: 1000,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
		})
	})

	t.Run("InnerTSID", func(t *testing.T) {
		tsids1 := []TSID{{MetricID: 1235}}
		tsids2 := []TSID{{MetricID: 1235}, {MetricID: 1236}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -100,
				MaxTimestamp: 1000,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
		})
	})

	t.Run("MatchingSmallerTSID", func(t *testing.T) {
		rbs := []rawBlock{{
			TSID: TSID{
				MetricID: 1234,
			},
			Timestamps: []int64{100},
			Values:     []float64{345},
		}}

		tsids1 := []TSID{{MetricID: 1234}}
		tsids2 := []TSID{{MetricID: 1234}, {MetricID: 1235}, {MetricID: 12345}}
		tsids3 := []TSID{{MetricID: 10}, {MetricID: 1234}}
		tsids4 := []TSID{{MetricID: 10}, {MetricID: 1234}, {MetricID: 12345}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -100,
				MaxTimestamp: 1000,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})
	})

	t.Run("MatchingBiggerTSID", func(t *testing.T) {
		rbs := []rawBlock{{
			TSID: TSID{
				MetricID: 2345,
			},
			Timestamps: []int64{200},
			Values:     []float64{456},
		}}

		tsids1 := []TSID{{MetricID: 2345}}
		tsids2 := []TSID{{MetricID: 2345}, {MetricID: 2346}, {MetricID: 12345}}
		tsids3 := []TSID{{MetricID: 10}, {MetricID: 2345}}
		tsids4 := []TSID{{MetricID: 10}, {MetricID: 2345}, {MetricID: 12345}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -100,
				MaxTimestamp: 1000,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 200,
				MaxTimestamp: 200,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})
	})

	t.Run("MatchingTwoTSIDs", func(t *testing.T) {
		rbs := []rawBlock{
			{
				TSID: TSID{
					MetricID: 1234,
				},
				Timestamps: []int64{100},
				Values:     []float64{345},
			},
			{
				TSID: TSID{
					MetricID: 2345,
				},
				Timestamps: []int64{200},
				Values:     []float64{456},
			},
		}

		tsids1 := []TSID{{MetricID: 1234}, {MetricID: 2345}}
		tsids2 := []TSID{{MetricID: 1234}, {MetricID: 2345}, {MetricID: 2346}, {MetricID: 12345}}
		tsids3 := []TSID{{MetricID: 10}, {MetricID: 1234}, {MetricID: 2345}}
		tsids4 := []TSID{{MetricID: 10}, {MetricID: 1234}, {MetricID: 1235}, {MetricID: 2345}, {MetricID: 12345}}

		t.Run("OuterTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -100,
				MaxTimestamp: 1000,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("ExactTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 200,
			}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("LowerEndTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 100,
				MaxTimestamp: 100,
			}
			rbs := []rawBlock{{
				TSID: TSID{
					MetricID: 1234,
				},
				Timestamps: []int64{100},
				Values:     []float64{345},
			}}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("LowerIntersectTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 90,
				MaxTimestamp: 150,
			}
			rbs := []rawBlock{{
				TSID: TSID{
					MetricID: 1234,
				},
				Timestamps: []int64{100},
				Values:     []float64{345},
			}}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("HigherEndTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 200,
				MaxTimestamp: 200,
			}
			rbs := []rawBlock{{
				TSID: TSID{
					MetricID: 2345,
				},
				Timestamps: []int64{200},
				Values:     []float64{456},
			}}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("HigherIntersectTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 170,
				MaxTimestamp: 250,
			}
			rbs := []rawBlock{{
				TSID: TSID{
					MetricID: 2345,
				},
				Timestamps: []int64{200},
				Values:     []float64{456},
			}}
			testPartSearch(t, p, tsids1, tr, rbs)
			testPartSearch(t, p, tsids2, tr, rbs)
			testPartSearch(t, p, tsids3, tr, rbs)
			testPartSearch(t, p, tsids4, tr, rbs)
		})

		t.Run("IvalidTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 201,
				MaxTimestamp: 99,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("InnerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 101,
				MaxTimestamp: 199,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("LowerTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: -2e6,
				MaxTimestamp: -1e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})

		t.Run("HigherTimeRange", func(t *testing.T) {
			tr := TimeRange{
				MinTimestamp: 1e6,
				MaxTimestamp: 2e6,
			}
			testPartSearch(t, p, tsids1, tr, []rawBlock{})
			testPartSearch(t, p, tsids2, tr, []rawBlock{})
			testPartSearch(t, p, tsids3, tr, []rawBlock{})
			testPartSearch(t, p, tsids4, tr, []rawBlock{})
		})
	})
}

func TestPartSearchMultiRowsOneTSID(t *testing.T) {
	for rowsCount := 1; rowsCount <= 1e5; rowsCount *= 10 {
		t.Run(fmt.Sprintf("Rows%d", rowsCount), func(t *testing.T) {
			testPartSearchMultiRowsOneTSID(t, rowsCount)
		})
	}
}

func testPartSearchMultiRowsOneTSID(t *testing.T, rowsCount int) {
	t.Helper()

	var rows []rawRow
	var r rawRow
	r.PrecisionBits = 24
	r.TSID.MetricID = 1111
	for i := 0; i < rowsCount; i++ {
		r.Timestamp = int64(rand.NormFloat64() * 1e6)
		r.Value = float64(int(rand.NormFloat64() * 1e5))
		rows = append(rows, r)
	}

	tsids := []TSID{{MetricID: 1111}}
	tr := TimeRange{
		MinTimestamp: -1e5,
		MaxTimestamp: 1e5,
	}
	expectedRawBlocks := getTestExpectedRawBlocks(rows, tsids, tr)
	p := newTestPart(rows)

	testPartSearch(t, p, tsids, tr, expectedRawBlocks)
}

func TestPartSearchMultiRowsMultiTSIDs(t *testing.T) {
	for rowsCount := 1; rowsCount <= 1e5; rowsCount *= 10 {
		t.Run(fmt.Sprintf("Rows%d", rowsCount), func(t *testing.T) {
			for tsidsCount := 1; tsidsCount <= rowsCount; tsidsCount *= 10 {
				t.Run(fmt.Sprintf("TSIDs%d", tsidsCount), func(t *testing.T) {
					testPartSearchMultiRowsMultiTSIDs(t, rowsCount, tsidsCount)
				})
			}
		})
	}
}

func testPartSearchMultiRowsMultiTSIDs(t *testing.T, rowsCount, tsidsCount int) {
	t.Helper()

	var rows []rawRow
	var r rawRow
	r.PrecisionBits = 24
	for i := 0; i < rowsCount; i++ {
		r.TSID.MetricID = uint64(rand.Intn(tsidsCount))
		r.Timestamp = int64(rand.NormFloat64() * 1e6)
		r.Value = float64(int(rand.NormFloat64() * 1e5))
		rows = append(rows, r)
	}

	var tsids []TSID
	var tsid TSID
	for i := 0; i < 100; i++ {
		tsid.MetricID = uint64(rand.Intn(tsidsCount * 3))
		tsids = append(tsids, tsid)
	}
	sort.Slice(tsids, func(i, j int) bool { return tsids[i].Less(&tsids[j]) })
	tr := TimeRange{
		MinTimestamp: -1e5,
		MaxTimestamp: 1e5,
	}
	expectedRawBlocks := getTestExpectedRawBlocks(rows, tsids, tr)
	p := newTestPart(rows)

	testPartSearch(t, p, tsids, tr, expectedRawBlocks)
}

func testPartSearch(t *testing.T, p *part, tsids []TSID, tr TimeRange, expectedRawBlocks []rawBlock) {
	t.Helper()

	if err := testPartSearchSerial(p, tsids, tr, expectedRawBlocks); err != nil {
		t.Fatalf("unexpected error in serial part search: %s", err)
	}

	// Run concurrent part search.
	ch := make(chan error, 5)
	for i := 0; i < cap(ch); i++ {
		go func() {
			err := testPartSearchSerial(p, tsids, tr, expectedRawBlocks)
			ch <- err
		}()
	}
	for i := 0; i < cap(ch); i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error in concurrent part search: %s", err)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout in concurrent part search")
		}
	}
}

func testPartSearchSerial(p *part, tsids []TSID, tr TimeRange, expectedRawBlocks []rawBlock) error {
	var ps partSearch
	ps.Init(p, tsids, tr)
	var bs []Block
	for ps.NextBlock() {
		var b Block
		ps.BlockRef.MustReadBlock(&b, true)
		bs = append(bs, b)
	}
	if err := ps.Error(); err != nil {
		return fmt.Errorf("unexpected error in search: %w", err)
	}

	if bs == nil {
		bs = []Block{}
	}
	rbs := newTestRawBlocks(bs, tr)
	if err := testEqualRawBlocks(rbs, expectedRawBlocks); err != nil {
		return fmt.Errorf("unequal blocks: %w", err)
	}
	return nil
}

func testEqualRawBlocks(a, b []rawBlock) error {
	a = newTestMergeRawBlocks(a)
	b = newTestMergeRawBlocks(b)
	if len(a) != len(b) {
		return fmt.Errorf("blocks length mismatch: got %d; want %d", len(a), len(b))
	}
	for i := range a {
		rb1 := &a[i]
		rb2 := &b[i]
		if !reflect.DeepEqual(rb1, rb2) {
			return fmt.Errorf("blocks mismatch on position %d out of %d; got\n%+v; want\n%+v", i, len(a), rb1, rb2)
		}
	}
	return nil
}

func newTestRawBlocks(bs []Block, tr TimeRange) []rawBlock {
	rbs := make([]rawBlock, 0, len(bs))
	for i := range bs {
		rb := newTestRawBlock(&bs[i], tr)
		if len(rb.Values) > 0 {
			rbs = append(rbs, rb)
		}
	}
	return rbs
}

func newTestRawBlock(b *Block, tr TimeRange) rawBlock {
	if err := b.UnmarshalData(); err != nil {
		panic(fmt.Errorf("cannot unmarshal block data: %w", err))
	}
	var rb rawBlock
	var values []int64
	for b.nextRow() {
		timestamp := b.timestamps[b.nextIdx-1]
		value := b.values[b.nextIdx-1]
		if timestamp < tr.MinTimestamp {
			continue
		}
		if timestamp > tr.MaxTimestamp {
			break
		}
		rb.Timestamps = append(rb.Timestamps, timestamp)
		values = append(values, value)
	}
	rb.TSID = b.bh.TSID
	rb.Values = decimal.AppendDecimalToFloat(rb.Values[:0], values, b.bh.Scale)
	return rb
}

func newTestMergeRawBlocks(src []rawBlock) []rawBlock {
	dst := make([]rawBlock, 0, len(src))
	if len(src) == 0 {
		return dst
	}
	rb := &rawBlock{
		TSID: src[0].TSID,
	}
	for len(src) > 0 {
		if src[0].TSID.MetricID != rb.TSID.MetricID {
			sort.Sort(&rawBlockSort{rb})
			dst = append(dst, *rb)
			rb = &rawBlock{
				TSID: src[0].TSID,
			}
		}
		rb.Timestamps = append(rb.Timestamps, src[0].Timestamps...)
		rb.Values = append(rb.Values, src[0].Values...)
		src = src[1:]
	}
	sort.Sort(&rawBlockSort{rb})
	dst = append(dst, *rb)
	return dst
}

type rawBlockSort struct {
	rb *rawBlock
}

func (rbs rawBlockSort) Len() int { return len(rbs.rb.Timestamps) }
func (rbs *rawBlockSort) Less(i, j int) bool {
	rb := rbs.rb
	if rb.Timestamps[i] < rb.Timestamps[j] {
		return true
	}
	if rb.Timestamps[i] > rb.Timestamps[j] {
		return false
	}
	return rb.Values[i] < rb.Values[j]
}
func (rbs *rawBlockSort) Swap(i, j int) {
	rb := rbs.rb
	rb.Timestamps[i], rb.Timestamps[j] = rb.Timestamps[j], rb.Timestamps[i]
	rb.Values[i], rb.Values[j] = rb.Values[j], rb.Values[i]
}

func getTestExpectedRawBlocks(rowsOriginal []rawRow, tsids []TSID, tr TimeRange) []rawBlock {
	if len(rowsOriginal) == 0 {
		return []rawBlock{}
	}

	rows := append([]rawRow{}, rowsOriginal...)
	sort.Slice(rows, func(i, j int) bool {
		a, b := &rows[i], &rows[j]
		if a.TSID.Less(&b.TSID) {
			return true
		}
		if b.TSID.Less(&a.TSID) {
			return false
		}
		if a.Timestamp < b.Timestamp {
			return true
		}
		if a.Timestamp > b.Timestamp {
			return false
		}
		return a.Value < b.Value
	})

	tsidsMap := make(map[TSID]bool)
	for _, tsid := range tsids {
		tsidsMap[tsid] = true
	}

	expectedRawBlocks := []rawBlock{}
	var rb rawBlock
	rb.TSID = rows[0].TSID
	rowsPerBlock := 0
	for i := range rows {
		r := &rows[i]
		if r.TSID.MetricID != rb.TSID.MetricID || rowsPerBlock >= maxRowsPerBlock {
			if tsidsMap[rb.TSID] && len(rb.Timestamps) > 0 {
				var tmpRB rawBlock
				tmpRB.CopyFrom(&rb)
				expectedRawBlocks = append(expectedRawBlocks, tmpRB)
			}
			rb.Reset()
			rb.TSID = r.TSID
			rowsPerBlock = 0
		}
		rowsPerBlock++
		if r.Timestamp < tr.MinTimestamp || r.Timestamp > tr.MaxTimestamp {
			continue
		}
		rb.Timestamps = append(rb.Timestamps, r.Timestamp)
		rb.Values = append(rb.Values, r.Value)
	}
	if tsidsMap[rb.TSID] && len(rb.Timestamps) > 0 {
		expectedRawBlocks = append(expectedRawBlocks, rb)
	}
	return expectedRawBlocks
}

func newTestPart(rows []rawRow) *part {
	mp := newTestInmemoryPart(rows)
	p, err := mp.NewPart()
	if err != nil {
		panic(fmt.Errorf("cannot create new part: %w", err))
	}
	return p
}
