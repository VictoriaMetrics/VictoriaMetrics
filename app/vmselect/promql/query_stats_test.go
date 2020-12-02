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
			qst.insertQuery(fmt.Sprintf("random-n-%d", i), int64(i), time.Now().Add(-time.Second), 500+time.Duration(i))
		}
		if len(qst.s) != expectedLen {
			t.Fatalf("unxpected len got=%d, for queryStats slice, want=%d", len(qst.s), expectedLen)
		}
	}
	f(10, 5, 10)
	f(30, 5, 10)
	f(16, 5, 16)
}

func TestGetTopNQueriesByDuration(t *testing.T) {
	f := func(topN int, expectedQueryStats []queryStats) {
		t.Helper()
		ql := &queryStatsTracker{
			limit:                 25,
			maxQueryLogRecordTime: time.Second * 5,
		}
		for i := 0; i < 21; i += 1 {
			ql.insertQuery(fmt.Sprintf("query-n-%d", i%3), int64(0), time.Now(), time.Second*time.Duration(i))
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
	f(1, []queryStats{{query: "query-n-2"}})
	f(2, []queryStats{{query: "query-n-2"}, {query: "query-n-1"}})
}

func TestGetTopNQueriesByCount(t *testing.T) {
	f := func(topN int, expectedQueryStats []queryStats) {
		t.Helper()
		ql := &queryStatsTracker{
			limit:                 25,
			maxQueryLogRecordTime: time.Second * 5,
		}
		for i := 0; i < 21; i += 1 {
			ql.insertQuery(fmt.Sprintf("query-n-%d", i%3), int64(0), time.Now(), time.Second*time.Duration(i))
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
	f(1, []queryStats{{query: "query-n-0"}})
	f(2, []queryStats{{query: "query-n-0"}, {query: "query-n-1"}})
}

func TestGetTopNQueriesByAverageDuration(t *testing.T) {
	f := func(topN int, expectedQueryStats []queryStats) {
		t.Helper()
		ql := &queryStatsTracker{
			limit:                 25,
			maxQueryLogRecordTime: time.Second * 5,
		}
		for i := 0; i < 21; i += 1 {
			ql.insertQuery(fmt.Sprintf("query-n-%d", i%3), int64(0), time.Now(), time.Second*time.Duration(i))
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
	f(1, []queryStats{{query: "query-n-2"}})
	f(2, []queryStats{{query: "query-n-2"}, {query: "query-n-1"}})
}
