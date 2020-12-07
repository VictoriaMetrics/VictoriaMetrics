package promql

import (
	"fmt"
	"testing"
	"time"
)

func TestQueryLoggerShrink(t *testing.T) {
	f := func(addItemCount, limit, expectedLen int) {
		t.Helper()
		qst := &queryStatsTracker{
			limit:                 limit,
			maxQueryLogRecordTime: time.Second * 5,
		}
		for i := 0; i < addItemCount; i++ {
			qst.insertQueryStat(fmt.Sprintf("random-n-%d", i), int64(i), time.Now().Add(-time.Second), 500+time.Duration(i))
		}
		if len(qst.qs) != expectedLen {
			t.Fatalf("unxpected len got=%d, for queryStats slice, want=%d", len(qst.qs), expectedLen)
		}
	}
	f(10, 5, 6)
	f(30, 10, 11)
	f(15, 15, 15)
}

func TestGetTopNQueriesByDuration(t *testing.T) {
	f := func(topN int, expectedQueryStats []queryStats) {
		t.Helper()
		ql := &queryStatsTracker{
			limit:                 25,
			maxQueryLogRecordTime: time.Second * 5,
		}
		queriesDurations := []int{16, 4, 5, 10}
		for i, v := range queriesDurations {
			ql.insertQueryStat(fmt.Sprintf("query-n-%d", i), int64(0), time.Now(), time.Second*time.Duration(v))
		}
		got := getTopNQueriesByAvgDuration(ql, topN)

		if len(got) != len(expectedQueryStats) {
			t.Fatalf("unxpected len of result, got: %d, want: %d", len(got), len(expectedQueryStats))
		}
		for i, gotR := range got {
			if gotR.query != expectedQueryStats[i].query {
				t.Fatalf("unxpected query: %q at position: %d, want: %q", gotR.query, i, expectedQueryStats[i].query)
			}
		}
	}
	f(1, []queryStats{{query: "query-n-0"}})
	f(2, []queryStats{{query: "query-n-0"}, {query: "query-n-3"}})
}

func TestGetTopNQueriesByCount(t *testing.T) {
	f := func(topN int, expectedQueryStats []queryStats) {
		t.Helper()
		ql := &queryStatsTracker{
			limit:                 25,
			maxQueryLogRecordTime: time.Second * 5,
		}
		queriesCounts := []int{1, 4, 5, 11}
		for i, v := range queriesCounts {
			for ic := 0; ic < v; ic++ {
				ql.insertQueryStat(fmt.Sprintf("query-n-%d", i), int64(0), time.Now(), time.Second*time.Duration(v))
			}
		}

		got := getTopNQueriesByRecordCount(ql, topN)

		if len(got) != len(expectedQueryStats) {
			t.Fatalf("unxpected len of result, got: %d, want: %d", len(got), len(expectedQueryStats))
		}
		for i, gotR := range got {
			if gotR.query != expectedQueryStats[i].query {
				t.Fatalf("unxpected query: %q at position: %d, want: %q", gotR.query, i, expectedQueryStats[i].query)
			}
		}
	}
	f(1, []queryStats{{query: "query-n-3"}})
	f(2, []queryStats{{query: "query-n-3"}, {query: "query-n-2"}})
}

func TestGetTopNQueriesByAverageDuration(t *testing.T) {
	f := func(topN int, expectedQueryStats []queryStats) {
		t.Helper()
		ql := &queryStatsTracker{
			limit:                 25,
			maxQueryLogRecordTime: time.Second * 5,
		}
		queriesQurations := []int{4, 25, 14, 10}
		for i, v := range queriesQurations {
			ql.insertQueryStat(fmt.Sprintf("query-n-%d", i), int64(0), time.Now(), time.Second*time.Duration(v))
		}

		got := getTopNQueriesByAvgDuration(ql, topN)

		if len(got) != len(expectedQueryStats) {
			t.Fatalf("unxpected len of result, got: %d, want: %d", len(got), len(expectedQueryStats))
		}
		for i, gotR := range got {
			if gotR.query != expectedQueryStats[i].query {
				t.Fatalf("unxpected query: %q at position: %d, want: %q", gotR.query, i, expectedQueryStats[i].query)
			}
		}
	}
	f(1, []queryStats{{query: "query-n-1"}})
	f(2, []queryStats{{query: "query-n-1"}, {query: "query-n-2"}})
}
