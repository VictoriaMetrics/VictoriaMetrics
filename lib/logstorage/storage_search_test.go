package logstorage

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestStorageRunQuery(t *testing.T) {
	t.Parallel()

	path := t.Name()

	const tenantsCount = 11
	const streamsPerTenant = 3
	const blocksPerStream = 5
	const rowsPerBlock = 7

	sc := &StorageConfig{
		Retention: 24 * time.Hour,
	}
	s := MustOpenStorage(path, sc)

	// fill the storage with data
	var allTenantIDs []TenantID
	baseTimestamp := time.Now().UnixNano() - 3600*1e9
	var fields []Field
	streamTags := []string{
		"job",
		"instance",
	}
	for i := 0; i < tenantsCount; i++ {
		tenantID := TenantID{
			AccountID: uint32(i),
			ProjectID: uint32(10*i + 1),
		}
		allTenantIDs = append(allTenantIDs, tenantID)
		for j := 0; j < streamsPerTenant; j++ {
			streamIDValue := fmt.Sprintf("stream_id=%d", j)
			for k := 0; k < blocksPerStream; k++ {
				lr := GetLogRows(streamTags, nil)
				for m := 0; m < rowsPerBlock; m++ {
					timestamp := baseTimestamp + int64(m)*1e9 + int64(k)
					// Append stream fields
					fields = append(fields[:0], Field{
						Name:  "job",
						Value: "foobar",
					}, Field{
						Name:  "instance",
						Value: fmt.Sprintf("host-%d:234", j),
					})
					// append the remaining fields
					fields = append(fields, Field{
						Name:  "_msg",
						Value: fmt.Sprintf("log message %d at block %d", m, k),
					})
					fields = append(fields, Field{
						Name:  "source-file",
						Value: "/foo/bar/baz",
					})
					fields = append(fields, Field{
						Name:  "tenant.id",
						Value: tenantID.String(),
					})
					fields = append(fields, Field{
						Name:  "stream-id",
						Value: streamIDValue,
					})
					lr.MustAdd(tenantID, timestamp, fields)
				}
				s.MustAddRows(lr)
				PutLogRows(lr)
			}
		}
	}
	s.debugFlush()

	mustRunQuery := func(t *testing.T, tenantIDs []TenantID, q *Query, writeBlock WriteBlockFunc) {
		t.Helper()
		err := s.RunQuery(context.Background(), tenantIDs, q, writeBlock)
		if err != nil {
			t.Fatalf("unexpected error returned from the query [%s]: %s", q, err)
		}
	}

	// run tests on the storage data
	t.Run("missing-tenant", func(t *testing.T) {
		q := mustParseQuery(`"log message"`)
		tenantID := TenantID{
			AccountID: 0,
			ProjectID: 0,
		}
		writeBlock := func(_ uint, timestamps []int64, _ []BlockColumn) {
			panic(fmt.Errorf("unexpected match for %d rows", len(timestamps)))
		}
		tenantIDs := []TenantID{tenantID}
		mustRunQuery(t, tenantIDs, q, writeBlock)
	})
	t.Run("missing-message-text", func(t *testing.T) {
		q := mustParseQuery(`foobar`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		writeBlock := func(_ uint, timestamps []int64, _ []BlockColumn) {
			panic(fmt.Errorf("unexpected match for %d rows", len(timestamps)))
		}
		tenantIDs := []TenantID{tenantID}
		mustRunQuery(t, tenantIDs, q, writeBlock)
	})
	t.Run("matching-tenant-id", func(t *testing.T) {
		q := mustParseQuery(`tenant.id:*`)
		for i := 0; i < tenantsCount; i++ {
			tenantID := TenantID{
				AccountID: uint32(i),
				ProjectID: uint32(10*i + 1),
			}
			expectedTenantID := tenantID.String()
			var rowsCountTotal atomic.Uint32
			writeBlock := func(_ uint, timestamps []int64, columns []BlockColumn) {
				hasTenantIDColumn := false
				var columnNames []string
				for _, c := range columns {
					if c.Name == "tenant.id" {
						hasTenantIDColumn = true
						if len(c.Values) != len(timestamps) {
							panic(fmt.Errorf("unexpected number of rows in column %q; got %d; want %d", c.Name, len(c.Values), len(timestamps)))
						}
						for _, v := range c.Values {
							if v != expectedTenantID {
								panic(fmt.Errorf("unexpected tenant.id; got %s; want %s", v, expectedTenantID))
							}
						}
					}
					columnNames = append(columnNames, c.Name)
				}
				if !hasTenantIDColumn {
					panic(fmt.Errorf("missing tenant.id column among columns: %q", columnNames))
				}
				rowsCountTotal.Add(uint32(len(timestamps)))
			}
			tenantIDs := []TenantID{tenantID}
			mustRunQuery(t, tenantIDs, q, writeBlock)

			expectedRowsCount := streamsPerTenant * blocksPerStream * rowsPerBlock
			if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
				t.Fatalf("unexpected number of matching rows; got %d; want %d", n, expectedRowsCount)
			}
		}
	})
	t.Run("matching-multiple-tenant-ids", func(t *testing.T) {
		q := mustParseQuery(`"log message"`)
		var rowsCountTotal atomic.Uint32
		writeBlock := func(_ uint, timestamps []int64, _ []BlockColumn) {
			rowsCountTotal.Add(uint32(len(timestamps)))
		}
		mustRunQuery(t, allTenantIDs, q, writeBlock)

		expectedRowsCount := tenantsCount * streamsPerTenant * blocksPerStream * rowsPerBlock
		if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of matching rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-in-filter", func(t *testing.T) {
		q := mustParseQuery(`source-file:in(foobar,/foo/bar/baz)`)
		var rowsCountTotal atomic.Uint32
		writeBlock := func(_ uint, timestamps []int64, _ []BlockColumn) {
			rowsCountTotal.Add(uint32(len(timestamps)))
		}
		mustRunQuery(t, allTenantIDs, q, writeBlock)

		expectedRowsCount := tenantsCount * streamsPerTenant * blocksPerStream * rowsPerBlock
		if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of matching rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("stream-filter-mismatch", func(t *testing.T) {
		q := mustParseQuery(`_stream:{job="foobar",instance=~"host-.+:2345"} log`)
		writeBlock := func(_ uint, timestamps []int64, _ []BlockColumn) {
			panic(fmt.Errorf("unexpected match for %d rows", len(timestamps)))
		}
		mustRunQuery(t, allTenantIDs, q, writeBlock)
	})
	t.Run("matching-stream-id", func(t *testing.T) {
		for i := 0; i < streamsPerTenant; i++ {
			q := mustParseQuery(fmt.Sprintf(`log _stream:{job="foobar",instance="host-%d:234"} AND stream-id:*`, i))
			tenantID := TenantID{
				AccountID: 1,
				ProjectID: 11,
			}
			expectedStreamID := fmt.Sprintf("stream_id=%d", i)
			var rowsCountTotal atomic.Uint32
			writeBlock := func(_ uint, timestamps []int64, columns []BlockColumn) {
				hasStreamIDColumn := false
				var columnNames []string
				for _, c := range columns {
					if c.Name == "stream-id" {
						hasStreamIDColumn = true
						if len(c.Values) != len(timestamps) {
							panic(fmt.Errorf("unexpected number of rows for column %q; got %d; want %d", c.Name, len(c.Values), len(timestamps)))
						}
						for _, v := range c.Values {
							if v != expectedStreamID {
								panic(fmt.Errorf("unexpected stream-id; got %s; want %s", v, expectedStreamID))
							}
						}
					}
					columnNames = append(columnNames, c.Name)
				}
				if !hasStreamIDColumn {
					panic(fmt.Errorf("missing stream-id column among columns: %q", columnNames))
				}
				rowsCountTotal.Add(uint32(len(timestamps)))
			}
			tenantIDs := []TenantID{tenantID}
			mustRunQuery(t, tenantIDs, q, writeBlock)

			expectedRowsCount := blocksPerStream * rowsPerBlock
			if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
				t.Fatalf("unexpected number of rows for stream %d; got %d; want %d", i, n, expectedRowsCount)
			}
		}
	})
	t.Run("matching-multiple-stream-ids-with-re-filter", func(t *testing.T) {
		q := mustParseQuery(`_msg:log _stream:{job="foobar",instance=~"host-[^:]+:234"} and re("message [02] at")`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		var rowsCountTotal atomic.Uint32
		writeBlock := func(_ uint, timestamps []int64, _ []BlockColumn) {
			rowsCountTotal.Add(uint32(len(timestamps)))
		}
		tenantIDs := []TenantID{tenantID}
		mustRunQuery(t, tenantIDs, q, writeBlock)

		expectedRowsCount := streamsPerTenant * blocksPerStream * 2
		if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-time-range", func(t *testing.T) {
		minTimestamp := baseTimestamp + (rowsPerBlock-2)*1e9
		maxTimestamp := baseTimestamp + (rowsPerBlock-1)*1e9 - 1
		q := mustParseQuery(fmt.Sprintf(`_time:[%f,%f]`, float64(minTimestamp)/1e9, float64(maxTimestamp)/1e9))
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		var rowsCountTotal atomic.Uint32
		writeBlock := func(_ uint, timestamps []int64, _ []BlockColumn) {
			rowsCountTotal.Add(uint32(len(timestamps)))
		}
		tenantIDs := []TenantID{tenantID}
		mustRunQuery(t, tenantIDs, q, writeBlock)

		expectedRowsCount := streamsPerTenant * blocksPerStream
		if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-stream-id-with-time-range", func(t *testing.T) {
		minTimestamp := baseTimestamp + (rowsPerBlock-2)*1e9
		maxTimestamp := baseTimestamp + (rowsPerBlock-1)*1e9 - 1
		q := mustParseQuery(fmt.Sprintf(`_time:[%f,%f] _stream:{job="foobar",instance="host-1:234"}`, float64(minTimestamp)/1e9, float64(maxTimestamp)/1e9))
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		var rowsCountTotal atomic.Uint32
		writeBlock := func(_ uint, timestamps []int64, _ []BlockColumn) {
			rowsCountTotal.Add(uint32(len(timestamps)))
		}
		tenantIDs := []TenantID{tenantID}
		mustRunQuery(t, tenantIDs, q, writeBlock)

		expectedRowsCount := blocksPerStream
		if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-stream-id-missing-time-range", func(t *testing.T) {
		minTimestamp := baseTimestamp + (rowsPerBlock+1)*1e9
		maxTimestamp := baseTimestamp + (rowsPerBlock+2)*1e9
		q := mustParseQuery(fmt.Sprintf(`_stream:{job="foobar",instance="host-1:234"} _time:[%d, %d)`, minTimestamp/1e9, maxTimestamp/1e9))
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		writeBlock := func(_ uint, timestamps []int64, _ []BlockColumn) {
			panic(fmt.Errorf("unexpected match for %d rows", len(timestamps)))
		}
		tenantIDs := []TenantID{tenantID}
		mustRunQuery(t, tenantIDs, q, writeBlock)
	})
	t.Run("missing-time-range", func(t *testing.T) {
		minTimestamp := baseTimestamp + (rowsPerBlock+1)*1e9
		maxTimestamp := baseTimestamp + (rowsPerBlock+2)*1e9
		q := mustParseQuery(fmt.Sprintf(`_time:[%d, %d)`, minTimestamp/1e9, maxTimestamp/1e9))
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		writeBlock := func(_ uint, timestamps []int64, _ []BlockColumn) {
			panic(fmt.Errorf("unexpected match for %d rows", len(timestamps)))
		}
		tenantIDs := []TenantID{tenantID}
		mustRunQuery(t, tenantIDs, q, writeBlock)
	})
	t.Run("field_names-all", func(t *testing.T) {
		q := mustParseQuery("*")
		results, err := s.GetFieldNames(context.Background(), allTenantIDs, q)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		resultsExpected := []ValueWithHits{
			{"_msg", 1155},
			{"_stream", 1155},
			{"_stream_id", 1155},
			{"_time", 1155},
			{"instance", 1155},
			{"job", 1155},
			{"source-file", 1155},
			{"stream-id", 1155},
			{"tenant.id", 1155},
		}
		if !reflect.DeepEqual(results, resultsExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", results, resultsExpected)
		}
	})
	t.Run("field_names-some", func(t *testing.T) {
		q := mustParseQuery(`_stream:{instance=~"host-1:.+"}`)
		results, err := s.GetFieldNames(context.Background(), allTenantIDs, q)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		resultsExpected := []ValueWithHits{
			{"_msg", 385},
			{"_stream", 385},
			{"_stream_id", 385},
			{"_time", 385},
			{"instance", 385},
			{"job", 385},
			{"source-file", 385},
			{"stream-id", 385},
			{"tenant.id", 385},
		}
		if !reflect.DeepEqual(results, resultsExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", results, resultsExpected)
		}
	})
	t.Run("field_values-nolimit", func(t *testing.T) {
		q := mustParseQuery("*")
		results, err := s.GetFieldValues(context.Background(), allTenantIDs, q, "_stream", 0)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		resultsExpected := []ValueWithHits{
			{`{instance="host-0:234",job="foobar"}`, 385},
			{`{instance="host-1:234",job="foobar"}`, 385},
			{`{instance="host-2:234",job="foobar"}`, 385},
		}
		if !reflect.DeepEqual(results, resultsExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", results, resultsExpected)
		}
	})
	t.Run("field_values-limit", func(t *testing.T) {
		q := mustParseQuery("*")
		results, err := s.GetFieldValues(context.Background(), allTenantIDs, q, "_stream", 3)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		resultsExpected := []ValueWithHits{
			{`{instance="host-0:234",job="foobar"}`, 385},
			{`{instance="host-1:234",job="foobar"}`, 385},
			{`{instance="host-2:234",job="foobar"}`, 385},
		}
		if !reflect.DeepEqual(results, resultsExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", results, resultsExpected)
		}
	})
	t.Run("field_values-limit", func(t *testing.T) {
		q := mustParseQuery("instance:='host-1:234'")
		results, err := s.GetFieldValues(context.Background(), allTenantIDs, q, "_stream", 4)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		resultsExpected := []ValueWithHits{
			{`{instance="host-1:234",job="foobar"}`, 385},
		}
		if !reflect.DeepEqual(results, resultsExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", results, resultsExpected)
		}
	})
	t.Run("stream_field_names", func(t *testing.T) {
		q := mustParseQuery("*")
		results, err := s.GetStreamFieldNames(context.Background(), allTenantIDs, q)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		resultsExpected := []ValueWithHits{
			{"instance", 1155},
			{"job", 1155},
		}
		if !reflect.DeepEqual(results, resultsExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", results, resultsExpected)
		}
	})
	t.Run("stream_field_values-nolimit", func(t *testing.T) {
		q := mustParseQuery("*")
		results, err := s.GetStreamFieldValues(context.Background(), allTenantIDs, q, "instance", 0)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		resultsExpected := []ValueWithHits{
			{`host-0:234`, 385},
			{`host-1:234`, 385},
			{`host-2:234`, 385},
		}
		if !reflect.DeepEqual(results, resultsExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", results, resultsExpected)
		}
	})
	t.Run("stream_field_values-limit", func(t *testing.T) {
		q := mustParseQuery("*")
		values, err := s.GetStreamFieldValues(context.Background(), allTenantIDs, q, "instance", 3)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		resultsExpected := []ValueWithHits{
			{`host-0:234`, 385},
			{`host-1:234`, 385},
			{`host-2:234`, 385},
		}
		if !reflect.DeepEqual(values, resultsExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", values, resultsExpected)
		}
	})
	t.Run("streams", func(t *testing.T) {
		q := mustParseQuery("*")
		results, err := s.GetStreams(context.Background(), allTenantIDs, q, 0)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		resultsExpected := []ValueWithHits{
			{`{instance="host-0:234",job="foobar"}`, 385},
			{`{instance="host-1:234",job="foobar"}`, 385},
			{`{instance="host-2:234",job="foobar"}`, 385},
		}
		if !reflect.DeepEqual(results, resultsExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", results, resultsExpected)
		}
	})
	t.Run("stream_ids", func(t *testing.T) {
		q := mustParseQuery("*")
		results, err := s.GetStreamIDs(context.Background(), allTenantIDs, q, 0)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// Verify the first 5 results with the smallest _stream_id value.
		sort.Slice(results, func(i, j int) bool {
			return results[i].Value < results[j].Value
		})
		results = results[:5]

		resultsExpected := []ValueWithHits{
			{"000000000000000140c1914be0226f8185f5b00551fb3b2d", 35},
			{"000000000000000177edafcd46385c778b57476eb5b92233", 35},
			{"0000000000000001f5b4cae620b5e85d6ef5f2107fe00274", 35},
			{"000000010000000b40c1914be0226f8185f5b00551fb3b2d", 35},
			{"000000010000000b77edafcd46385c778b57476eb5b92233", 35},
		}
		if !reflect.DeepEqual(results, resultsExpected) {
			t.Fatalf("unexpected result; got\n%v\nwant\n%v", results, resultsExpected)
		}
	})

	// Run more complex tests
	f := func(t *testing.T, query string, rowsExpected [][]Field) {
		t.Helper()

		q := mustParseQuery(query)
		var resultRowsLock sync.Mutex
		var resultRows [][]Field
		writeBlock := func(_ uint, _ []int64, bcs []BlockColumn) {
			if len(bcs) == 0 {
				return
			}

			for i := 0; i < len(bcs[0].Values); i++ {
				row := make([]Field, len(bcs))
				for j, bc := range bcs {
					row[j] = Field{
						Name:  strings.Clone(bc.Name),
						Value: strings.Clone(bc.Values[i]),
					}
				}
				resultRowsLock.Lock()
				resultRows = append(resultRows, row)
				resultRowsLock.Unlock()
			}
		}
		mustRunQuery(t, allTenantIDs, q, writeBlock)

		assertRowsEqual(t, resultRows, rowsExpected)
	}

	t.Run("stats-count-total", func(t *testing.T) {
		f(t, `* | stats count() rows`, [][]Field{
			{
				{"rows", "1155"},
			},
		})
	})
	t.Run("_stream_id-filter", func(t *testing.T) {
		f(t, `_stream_id:in(tenant.id:2 | fields _stream_id) | stats count() rows`, [][]Field{
			{
				{"rows", "105"},
			},
		})
	})
	t.Run("in-filter-with-subquery-match", func(t *testing.T) {
		f(t, `tenant.id:in(tenant.id:2 | fields tenant.id) | stats count() rows`, [][]Field{
			{
				{"rows", "105"},
			},
		})
	})
	t.Run("in-filter-with-subquery-mismatch", func(t *testing.T) {
		f(t, `tenant.id:in(tenant.id:23243 | fields tenant.id) | stats count() rows`, [][]Field{
			{
				{"rows", "0"},
			},
		})
	})
	t.Run("conditional-stats", func(t *testing.T) {
		f(t, `* | stats
			count() rows_total,
			count() if (stream-id:0) stream_0_rows,
			count() if (stream-id:1123) stream_x_rows
		`, [][]Field{
			{
				{"rows_total", "1155"},
				{"stream_0_rows", "385"},
				{"stream_x_rows", "0"},
			},
		})
	})
	t.Run("in-filter-with-subquery-in-conditional-stats-mismatch", func(t *testing.T) {
		f(t, `* | stats
			count() rows_total,
			count() if (tenant.id:in(tenant.id:3 | fields tenant.id)) rows_nonzero,
			count() if (tenant.id:in(tenant.id:23243 | fields tenant.id)) rows_zero
		`, [][]Field{
			{
				{"rows_total", "1155"},
				{"rows_nonzero", "105"},
				{"rows_zero", "0"},
			},
		})
	})
	t.Run("pipe-extract", func(t *testing.T) {
		f(t, `* | extract "host-<host>:" from instance | uniq (host) with hits | sort by (host)`, [][]Field{
			{
				{"host", "0"},
				{"hits", "385"},
			},
			{
				{"host", "1"},
				{"hits", "385"},
			},
			{
				{"host", "2"},
				{"hits", "385"},
			},
		})
	})
	t.Run("pipe-extract-if-filter-with-subquery", func(t *testing.T) {
		f(t, `* | extract
				if (tenant.id:in(tenant.id:(3 or 4) | fields tenant.id))
				"host-<host>:" from instance
			| filter host:~"1|2"
			| uniq (tenant.id, host) with hits
			| sort by (tenant.id, host)`, [][]Field{
			{
				{"tenant.id", "{accountID=3,projectID=31}"},
				{"host", "1"},
				{"hits", "35"},
			},
			{
				{"tenant.id", "{accountID=3,projectID=31}"},
				{"host", "2"},
				{"hits", "35"},
			},
			{
				{"tenant.id", "{accountID=4,projectID=41}"},
				{"host", "1"},
				{"hits", "35"},
			},
			{
				{"tenant.id", "{accountID=4,projectID=41}"},
				{"host", "2"},
				{"hits", "35"},
			},
		})
	})
	t.Run("pipe-extract-if-filter-with-subquery-non-empty-host", func(t *testing.T) {
		f(t, `* | extract
				if (tenant.id:in(tenant.id:3 | fields tenant.id))
				"host-<host>:" from instance
			| filter host:*
			| uniq (host) with hits
			| sort by (host)`, [][]Field{
			{
				{"host", "0"},
				{"hits", "35"},
			},
			{
				{"host", "1"},
				{"hits", "35"},
			},
			{
				{"host", "2"},
				{"hits", "35"},
			},
		})
	})
	t.Run("pipe-extract-if-filter-with-subquery-empty-host", func(t *testing.T) {
		f(t, `* | extract
				if (tenant.id:in(tenant.id:3 | fields tenant.id))
				"host-<host>:" from instance
			| filter host:""
			| uniq (host) with hits
			| sort by (host)`, [][]Field{
			{
				{"host", ""},
				{"hits", "1050"},
			},
		})
	})
	t.Run("stream_context-noop-1", func(t *testing.T) {
		f(t, `"message 3 at block 1"
			| stream_context before 0
			| stats count() rows`, [][]Field{
			{
				{"rows", "66"},
			},
		})
	})
	t.Run("stream_context-noop-2", func(t *testing.T) {
		f(t, `"message 3 at block 1"
			| stream_context before 0 after 0
			| stats count() rows`, [][]Field{
			{
				{"rows", "66"},
			},
		})
	})
	t.Run("stream_context-before-1", func(t *testing.T) {
		f(t, `"message 3 at block 1"
			| stream_context before 1
			| stats count() rows`, [][]Field{
			{
				{"rows", "99"},
			},
		})
	})
	t.Run("stream_context-after-1", func(t *testing.T) {
		f(t, `"message 3 at block 1"
			| stream_context after 1
			| stats count() rows`, [][]Field{
			{
				{"rows", "99"},
			},
		})
	})
	t.Run("stream_context-before-after-1", func(t *testing.T) {
		f(t, `"message 3 at block 1"
			| stream_context before 1 after 1
			| stats count() rows`, [][]Field{
			{
				{"rows", "132"},
			},
		})
	})
	t.Run("stream_context-before-1000", func(t *testing.T) {
		f(t, `"message 4"
			| stream_context before 1000
			| stats count() rows`, [][]Field{
			{
				{"rows", "990"},
			},
		})
	})
	t.Run("stream_context-after-1000", func(t *testing.T) {
		f(t, `"message 4"
			| stream_context after 1000
			| stats count() rows`, [][]Field{
			{
				{"rows", "660"},
			},
		})
	})
	t.Run("stream_context-before-after-1000", func(t *testing.T) {
		f(t, `"message 4"
			| stream_context before 1000 after 1000
			| stats count() rows`, [][]Field{
			{
				{"rows", "1320"},
			},
		})
	})

	// Close the storage and delete its data
	s.MustClose()
	fs.MustRemoveAll(path)
}

func mustParseQuery(query string) *Query {
	q, err := ParseQuery(query)
	if err != nil {
		panic(fmt.Errorf("BUG: cannot parse [%s]: %w", query, err))
	}
	return q
}

func TestStorageSearch(t *testing.T) {
	t.Parallel()

	path := t.Name()

	const tenantsCount = 11
	const streamsPerTenant = 3
	const blocksPerStream = 5
	const rowsPerBlock = 7

	sc := &StorageConfig{
		Retention: 24 * time.Hour,
	}
	s := MustOpenStorage(path, sc)

	// fill the storage with data.
	var allTenantIDs []TenantID
	baseTimestamp := time.Now().UnixNano() - 3600*1e9
	var fields []Field
	streamTags := []string{
		"job",
		"instance",
	}
	for i := 0; i < tenantsCount; i++ {
		tenantID := TenantID{
			AccountID: uint32(i),
			ProjectID: uint32(10*i + 1),
		}
		allTenantIDs = append(allTenantIDs, tenantID)
		for j := 0; j < streamsPerTenant; j++ {
			for k := 0; k < blocksPerStream; k++ {
				lr := GetLogRows(streamTags, nil)
				for m := 0; m < rowsPerBlock; m++ {
					timestamp := baseTimestamp + int64(m)*1e9 + int64(k)
					// Append stream fields
					fields = append(fields[:0], Field{
						Name:  "job",
						Value: "foobar",
					}, Field{
						Name:  "instance",
						Value: fmt.Sprintf("host-%d:234", j),
					})
					// append the remaining fields
					fields = append(fields, Field{
						Name:  "_msg",
						Value: fmt.Sprintf("log message %d at block %d", m, k),
					})
					fields = append(fields, Field{
						Name:  "source-file",
						Value: "/foo/bar/baz",
					})
					lr.MustAdd(tenantID, timestamp, fields)
				}
				s.MustAddRows(lr)
				PutLogRows(lr)
			}
		}
	}
	s.debugFlush()

	// run tests on the filled storage
	const workersCount = 3

	getBaseFilter := func(minTimestamp, maxTimestamp int64, sf *StreamFilter) filter {
		var filters []filter
		filters = append(filters, &filterTime{
			minTimestamp: minTimestamp,
			maxTimestamp: maxTimestamp,
		})
		if sf != nil {
			filters = append(filters, &filterStream{
				f: sf,
			})
		}
		return &filterAnd{
			filters: filters,
		}
	}

	t.Run("missing-tenant-smaller-than-existing", func(_ *testing.T) {
		tenantID := TenantID{
			AccountID: 0,
			ProjectID: 0,
		}
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, nil)
		so := newTestGenericSearchOptions([]TenantID{tenantID}, f, []string{"_msg"})
		processBlock := func(_ uint, _ *blockResult) {
			panic(fmt.Errorf("unexpected match"))
		}
		s.search(workersCount, so, nil, processBlock)
	})
	t.Run("missing-tenant-bigger-than-existing", func(_ *testing.T) {
		tenantID := TenantID{
			AccountID: tenantsCount + 1,
			ProjectID: 0,
		}
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, nil)
		so := newTestGenericSearchOptions([]TenantID{tenantID}, f, []string{"_msg"})
		processBlock := func(_ uint, _ *blockResult) {
			panic(fmt.Errorf("unexpected match"))
		}
		s.search(workersCount, so, nil, processBlock)
	})
	t.Run("missing-tenant-middle", func(_ *testing.T) {
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 0,
		}
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, nil)
		so := newTestGenericSearchOptions([]TenantID{tenantID}, f, []string{"_msg"})
		processBlock := func(_ uint, _ *blockResult) {
			panic(fmt.Errorf("unexpected match"))
		}
		s.search(workersCount, so, nil, processBlock)
	})
	t.Run("matching-tenant-id", func(t *testing.T) {
		for i := 0; i < tenantsCount; i++ {
			tenantID := TenantID{
				AccountID: uint32(i),
				ProjectID: uint32(10*i + 1),
			}
			minTimestamp := baseTimestamp
			maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
			f := getBaseFilter(minTimestamp, maxTimestamp, nil)
			so := newTestGenericSearchOptions([]TenantID{tenantID}, f, []string{"_msg"})
			var rowsCountTotal atomic.Uint32
			processBlock := func(_ uint, br *blockResult) {
				rowsCountTotal.Add(uint32(br.rowsLen))
			}
			s.search(workersCount, so, nil, processBlock)

			expectedRowsCount := streamsPerTenant * blocksPerStream * rowsPerBlock
			if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
				t.Fatalf("unexpected number of matching rows; got %d; want %d", n, expectedRowsCount)
			}
		}
	})
	t.Run("matching-multiple-tenant-ids", func(t *testing.T) {
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, nil)
		so := newTestGenericSearchOptions(allTenantIDs, f, []string{"_msg"})
		var rowsCountTotal atomic.Uint32
		processBlock := func(_ uint, br *blockResult) {
			rowsCountTotal.Add(uint32(br.rowsLen))
		}
		s.search(workersCount, so, nil, processBlock)

		expectedRowsCount := tenantsCount * streamsPerTenant * blocksPerStream * rowsPerBlock
		if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of matching rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("stream-filter-mismatch", func(_ *testing.T) {
		sf := mustNewTestStreamFilter(`{job="foobar",instance=~"host-.+:2345"}`)
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, sf)
		so := newTestGenericSearchOptions(allTenantIDs, f, []string{"_msg"})
		processBlock := func(_ uint, _ *blockResult) {
			panic(fmt.Errorf("unexpected match"))
		}
		s.search(workersCount, so, nil, processBlock)
	})
	t.Run("matching-stream-id", func(t *testing.T) {
		for i := 0; i < streamsPerTenant; i++ {
			sf := mustNewTestStreamFilter(fmt.Sprintf(`{job="foobar",instance="host-%d:234"}`, i))
			tenantID := TenantID{
				AccountID: 1,
				ProjectID: 11,
			}
			minTimestamp := baseTimestamp
			maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
			f := getBaseFilter(minTimestamp, maxTimestamp, sf)
			so := newTestGenericSearchOptions([]TenantID{tenantID}, f, []string{"_msg"})
			var rowsCountTotal atomic.Uint32
			processBlock := func(_ uint, br *blockResult) {
				rowsCountTotal.Add(uint32(br.rowsLen))
			}
			s.search(workersCount, so, nil, processBlock)

			expectedRowsCount := blocksPerStream * rowsPerBlock
			if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
				t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
			}
		}
	})
	t.Run("matching-multiple-stream-ids", func(t *testing.T) {
		sf := mustNewTestStreamFilter(`{job="foobar",instance=~"host-[^:]+:234"}`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, sf)
		so := newTestGenericSearchOptions([]TenantID{tenantID}, f, []string{"_msg"})
		var rowsCountTotal atomic.Uint32
		processBlock := func(_ uint, br *blockResult) {
			rowsCountTotal.Add(uint32(br.rowsLen))
		}
		s.search(workersCount, so, nil, processBlock)

		expectedRowsCount := streamsPerTenant * blocksPerStream * rowsPerBlock
		if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-multiple-stream-ids-with-re-filter", func(t *testing.T) {
		sf := mustNewTestStreamFilter(`{job="foobar",instance=~"host-[^:]+:234"}`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, sf)
		f = &filterAnd{
			filters: []filter{
				f,
				&filterRegexp{
					fieldName: "_msg",
					re:        mustCompileRegex("message [02] at "),
				},
			},
		}
		so := newTestGenericSearchOptions([]TenantID{tenantID}, f, []string{"_msg"})
		var rowsCountTotal atomic.Uint32
		processBlock := func(_ uint, br *blockResult) {
			rowsCountTotal.Add(uint32(br.rowsLen))
		}
		s.search(workersCount, so, nil, processBlock)

		expectedRowsCount := streamsPerTenant * blocksPerStream * 2
		if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-stream-id-smaller-time-range", func(t *testing.T) {
		sf := mustNewTestStreamFilter(`{job="foobar",instance="host-1:234"}`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		minTimestamp := baseTimestamp + (rowsPerBlock-2)*1e9
		maxTimestamp := baseTimestamp + (rowsPerBlock-1)*1e9 - 1
		f := getBaseFilter(minTimestamp, maxTimestamp, sf)
		so := newTestGenericSearchOptions([]TenantID{tenantID}, f, []string{"_msg"})
		var rowsCountTotal atomic.Uint32
		processBlock := func(_ uint, br *blockResult) {
			rowsCountTotal.Add(uint32(br.rowsLen))
		}
		s.search(workersCount, so, nil, processBlock)

		expectedRowsCount := blocksPerStream
		if n := rowsCountTotal.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-stream-id-missing-time-range", func(_ *testing.T) {
		sf := mustNewTestStreamFilter(`{job="foobar",instance="host-1:234"}`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		minTimestamp := baseTimestamp + (rowsPerBlock+1)*1e9
		maxTimestamp := baseTimestamp + (rowsPerBlock+2)*1e9
		f := getBaseFilter(minTimestamp, maxTimestamp, sf)
		so := newTestGenericSearchOptions([]TenantID{tenantID}, f, []string{"_msg"})
		processBlock := func(_ uint, _ *blockResult) {
			panic(fmt.Errorf("unexpected match"))
		}
		s.search(workersCount, so, nil, processBlock)
	})

	s.MustClose()
	fs.MustRemoveAll(path)
}

func TestParseStreamFieldsSuccess(t *testing.T) {
	t.Parallel()

	f := func(s, resultExpected string) {
		t.Helper()

		labels, err := parseStreamFields(nil, s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := MarshalFieldsToJSON(nil, labels)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	f(`{}`, `{}`)
	f(`{foo="bar"}`, `{"foo":"bar"}`)
	f(`{a="b",c="d"}`, `{"a":"b","c":"d"}`)
	f(`{a="a=,b\"c}",b="d"}`, `{"a":"a=,b\"c}","b":"d"}`)
}

func newTestGenericSearchOptions(tenantIDs []TenantID, f filter, neededColumns []string) *genericSearchOptions {
	return &genericSearchOptions{
		tenantIDs:         tenantIDs,
		minTimestamp:      math.MinInt64,
		maxTimestamp:      math.MaxInt64,
		filter:            f,
		neededColumnNames: neededColumns,
	}
}
