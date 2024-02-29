package logstorage

import (
	"fmt"
	"regexp"
	"sync/atomic"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestStorageRunQuery(t *testing.T) {
	const path = "TestStorageRunQuery"

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

	// run tests on the storage data
	t.Run("missing-tenant", func(t *testing.T) {
		q := mustParseQuery(`"log message"`)
		tenantID := TenantID{
			AccountID: 0,
			ProjectID: 0,
		}
		processBlock := func(columns []BlockColumn) {
			panic(fmt.Errorf("unexpected match"))
		}
		tenantIDs := []TenantID{tenantID}
		s.RunQuery(tenantIDs, q, nil, processBlock)
	})
	t.Run("missing-message-text", func(t *testing.T) {
		q := mustParseQuery(`foobar`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		processBlock := func(columns []BlockColumn) {
			panic(fmt.Errorf("unexpected match"))
		}
		tenantIDs := []TenantID{tenantID}
		s.RunQuery(tenantIDs, q, nil, processBlock)
	})
	t.Run("matching-tenant-id", func(t *testing.T) {
		q := mustParseQuery(`tenant.id:*`)
		for i := 0; i < tenantsCount; i++ {
			tenantID := TenantID{
				AccountID: uint32(i),
				ProjectID: uint32(10*i + 1),
			}
			expectedTenantID := tenantID.String()
			var rowsCount atomic.Uint32
			processBlock := func(columns []BlockColumn) {
				hasTenantIDColumn := false
				var columnNames []string
				for _, c := range columns {
					if c.Name == "tenant.id" {
						hasTenantIDColumn = true
						if len(c.Values) == 0 {
							panic(fmt.Errorf("unexpected zero rows"))
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
				rowsCount.Add(uint32(len(columns[0].Values)))
			}
			tenantIDs := []TenantID{tenantID}
			s.RunQuery(tenantIDs, q, nil, processBlock)

			expectedRowsCount := streamsPerTenant * blocksPerStream * rowsPerBlock
			if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
				t.Fatalf("unexpected number of matching rows; got %d; want %d", n, expectedRowsCount)
			}
		}
	})
	t.Run("matching-multiple-tenant-ids", func(t *testing.T) {
		q := mustParseQuery(`"log message"`)
		var rowsCount atomic.Uint32
		processBlock := func(columns []BlockColumn) {
			rowsCount.Add(uint32(len(columns[0].Values)))
		}
		s.RunQuery(allTenantIDs, q, nil, processBlock)

		expectedRowsCount := tenantsCount * streamsPerTenant * blocksPerStream * rowsPerBlock
		if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of matching rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-in-filter", func(t *testing.T) {
		q := mustParseQuery(`source-file:in(foobar,/foo/bar/baz)`)
		var rowsCount atomic.Uint32
		processBlock := func(columns []BlockColumn) {
			rowsCount.Add(uint32(len(columns[0].Values)))
		}
		s.RunQuery(allTenantIDs, q, nil, processBlock)

		expectedRowsCount := tenantsCount * streamsPerTenant * blocksPerStream * rowsPerBlock
		if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of matching rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("stream-filter-mismatch", func(t *testing.T) {
		q := mustParseQuery(`_stream:{job="foobar",instance=~"host-.+:2345"} log`)
		processBlock := func(columns []BlockColumn) {
			panic(fmt.Errorf("unexpected match"))
		}
		s.RunQuery(allTenantIDs, q, nil, processBlock)
	})
	t.Run("matching-stream-id", func(t *testing.T) {
		for i := 0; i < streamsPerTenant; i++ {
			q := mustParseQuery(fmt.Sprintf(`log _stream:{job="foobar",instance="host-%d:234"} AND stream-id:*`, i))
			tenantID := TenantID{
				AccountID: 1,
				ProjectID: 11,
			}
			expectedStreamID := fmt.Sprintf("stream_id=%d", i)
			var rowsCount atomic.Uint32
			processBlock := func(columns []BlockColumn) {
				hasStreamIDColumn := false
				var columnNames []string
				for _, c := range columns {
					if c.Name == "stream-id" {
						hasStreamIDColumn = true
						if len(c.Values) == 0 {
							panic(fmt.Errorf("unexpected zero rows"))
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
				rowsCount.Add(uint32(len(columns[0].Values)))
			}
			tenantIDs := []TenantID{tenantID}
			s.RunQuery(tenantIDs, q, nil, processBlock)

			expectedRowsCount := blocksPerStream * rowsPerBlock
			if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
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
		var rowsCount atomic.Uint32
		processBlock := func(columns []BlockColumn) {
			rowsCount.Add(uint32(len(columns[0].Values)))
		}
		tenantIDs := []TenantID{tenantID}
		s.RunQuery(tenantIDs, q, nil, processBlock)

		expectedRowsCount := streamsPerTenant * blocksPerStream * 2
		if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
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
		var rowsCount atomic.Uint32
		processBlock := func(columns []BlockColumn) {
			rowsCount.Add(uint32(len(columns[0].Values)))
		}
		tenantIDs := []TenantID{tenantID}
		s.RunQuery(tenantIDs, q, nil, processBlock)

		expectedRowsCount := streamsPerTenant * blocksPerStream
		if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
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
		var rowsCount atomic.Uint32
		processBlock := func(columns []BlockColumn) {
			rowsCount.Add(uint32(len(columns[0].Values)))
		}
		tenantIDs := []TenantID{tenantID}
		s.RunQuery(tenantIDs, q, nil, processBlock)

		expectedRowsCount := blocksPerStream
		if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
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
		processBlock := func(columns []BlockColumn) {
			panic(fmt.Errorf("unexpected match"))
		}
		tenantIDs := []TenantID{tenantID}
		s.RunQuery(tenantIDs, q, nil, processBlock)
	})
	t.Run("missing-time-range", func(t *testing.T) {
		minTimestamp := baseTimestamp + (rowsPerBlock+1)*1e9
		maxTimestamp := baseTimestamp + (rowsPerBlock+2)*1e9
		q := mustParseQuery(fmt.Sprintf(`_time:[%d, %d)`, minTimestamp/1e9, maxTimestamp/1e9))
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		processBlock := func(columns []BlockColumn) {
			panic(fmt.Errorf("unexpected match"))
		}
		tenantIDs := []TenantID{tenantID}
		s.RunQuery(tenantIDs, q, nil, processBlock)
	})

	// Close the storage and delete its data
	s.MustClose()
	fs.MustRemoveAll(path)
}

func mustParseQuery(query string) *Query {
	q, err := ParseQuery(query)
	if err != nil {
		panic(fmt.Errorf("BUG: cannot parse %s: %w", query, err))
	}
	return q
}

func TestStorageSearch(t *testing.T) {
	const path = "TestStorageSearch"

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
		filters = append(filters, &timeFilter{
			minTimestamp: minTimestamp,
			maxTimestamp: maxTimestamp,
		})
		if sf != nil {
			filters = append(filters, &streamFilter{
				f: sf,
			})
		}
		return &andFilter{
			filters: filters,
		}
	}

	t.Run("missing-tenant-smaller-than-existing", func(t *testing.T) {
		tenantID := TenantID{
			AccountID: 0,
			ProjectID: 0,
		}
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, nil)
		so := &genericSearchOptions{
			tenantIDs:         []TenantID{tenantID},
			filter:            f,
			resultColumnNames: []string{"_msg"},
		}
		processBlock := func(workerID uint, br *blockResult) {
			panic(fmt.Errorf("unexpected match"))
		}
		s.search(workersCount, so, nil, processBlock)
	})
	t.Run("missing-tenant-bigger-than-existing", func(t *testing.T) {
		tenantID := TenantID{
			AccountID: tenantsCount + 1,
			ProjectID: 0,
		}
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, nil)
		so := &genericSearchOptions{
			tenantIDs:         []TenantID{tenantID},
			filter:            f,
			resultColumnNames: []string{"_msg"},
		}
		processBlock := func(workerID uint, br *blockResult) {
			panic(fmt.Errorf("unexpected match"))
		}
		s.search(workersCount, so, nil, processBlock)
	})
	t.Run("missing-tenant-middle", func(t *testing.T) {
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 0,
		}
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, nil)
		so := &genericSearchOptions{
			tenantIDs:         []TenantID{tenantID},
			filter:            f,
			resultColumnNames: []string{"_msg"},
		}
		processBlock := func(workerID uint, br *blockResult) {
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
			so := &genericSearchOptions{
				tenantIDs:         []TenantID{tenantID},
				filter:            f,
				resultColumnNames: []string{"_msg"},
			}
			var rowsCount atomic.Uint32
			processBlock := func(workerID uint, br *blockResult) {
				if !br.streamID.tenantID.equal(&tenantID) {
					panic(fmt.Errorf("unexpected tenantID; got %s; want %s", &br.streamID.tenantID, &tenantID))
				}
				rowsCount.Add(uint32(br.RowsCount()))
			}
			s.search(workersCount, so, nil, processBlock)

			expectedRowsCount := streamsPerTenant * blocksPerStream * rowsPerBlock
			if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
				t.Fatalf("unexpected number of matching rows; got %d; want %d", n, expectedRowsCount)
			}
		}
	})
	t.Run("matching-multiple-tenant-ids", func(t *testing.T) {
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, nil)
		so := &genericSearchOptions{
			tenantIDs:         allTenantIDs,
			filter:            f,
			resultColumnNames: []string{"_msg"},
		}
		var rowsCount atomic.Uint32
		processBlock := func(workerID uint, br *blockResult) {
			rowsCount.Add(uint32(br.RowsCount()))
		}
		s.search(workersCount, so, nil, processBlock)

		expectedRowsCount := tenantsCount * streamsPerTenant * blocksPerStream * rowsPerBlock
		if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of matching rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("stream-filter-mismatch", func(t *testing.T) {
		sf := mustNewStreamFilter(`{job="foobar",instance=~"host-.+:2345"}`)
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, sf)
		so := &genericSearchOptions{
			tenantIDs:         allTenantIDs,
			filter:            f,
			resultColumnNames: []string{"_msg"},
		}
		processBlock := func(workerID uint, br *blockResult) {
			panic(fmt.Errorf("unexpected match"))
		}
		s.search(workersCount, so, nil, processBlock)
	})
	t.Run("matching-stream-id", func(t *testing.T) {
		for i := 0; i < streamsPerTenant; i++ {
			sf := mustNewStreamFilter(fmt.Sprintf(`{job="foobar",instance="host-%d:234"}`, i))
			tenantID := TenantID{
				AccountID: 1,
				ProjectID: 11,
			}
			minTimestamp := baseTimestamp
			maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
			f := getBaseFilter(minTimestamp, maxTimestamp, sf)
			so := &genericSearchOptions{
				tenantIDs:         []TenantID{tenantID},
				filter:            f,
				resultColumnNames: []string{"_msg"},
			}
			var rowsCount atomic.Uint32
			processBlock := func(workerID uint, br *blockResult) {
				if !br.streamID.tenantID.equal(&tenantID) {
					panic(fmt.Errorf("unexpected tenantID; got %s; want %s", &br.streamID.tenantID, &tenantID))
				}
				rowsCount.Add(uint32(br.RowsCount()))
			}
			s.search(workersCount, so, nil, processBlock)

			expectedRowsCount := blocksPerStream * rowsPerBlock
			if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
				t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
			}
		}
	})
	t.Run("matching-multiple-stream-ids", func(t *testing.T) {
		sf := mustNewStreamFilter(`{job="foobar",instance=~"host-[^:]+:234"}`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, sf)
		so := &genericSearchOptions{
			tenantIDs:         []TenantID{tenantID},
			filter:            f,
			resultColumnNames: []string{"_msg"},
		}
		var rowsCount atomic.Uint32
		processBlock := func(workerID uint, br *blockResult) {
			if !br.streamID.tenantID.equal(&tenantID) {
				panic(fmt.Errorf("unexpected tenantID; got %s; want %s", &br.streamID.tenantID, &tenantID))
			}
			rowsCount.Add(uint32(br.RowsCount()))
		}
		s.search(workersCount, so, nil, processBlock)

		expectedRowsCount := streamsPerTenant * blocksPerStream * rowsPerBlock
		if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-multiple-stream-ids-with-re-filter", func(t *testing.T) {
		sf := mustNewStreamFilter(`{job="foobar",instance=~"host-[^:]+:234"}`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		minTimestamp := baseTimestamp
		maxTimestamp := baseTimestamp + rowsPerBlock*1e9 + blocksPerStream
		f := getBaseFilter(minTimestamp, maxTimestamp, sf)
		f = &andFilter{
			filters: []filter{
				f,
				&regexpFilter{
					fieldName: "_msg",
					re:        regexp.MustCompile("message [02] at "),
				},
			},
		}
		so := &genericSearchOptions{
			tenantIDs:         []TenantID{tenantID},
			filter:            f,
			resultColumnNames: []string{"_msg"},
		}
		var rowsCount atomic.Uint32
		processBlock := func(workerID uint, br *blockResult) {
			if !br.streamID.tenantID.equal(&tenantID) {
				panic(fmt.Errorf("unexpected tenantID; got %s; want %s", &br.streamID.tenantID, &tenantID))
			}
			rowsCount.Add(uint32(br.RowsCount()))
		}
		s.search(workersCount, so, nil, processBlock)

		expectedRowsCount := streamsPerTenant * blocksPerStream * 2
		if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-stream-id-smaller-time-range", func(t *testing.T) {
		sf := mustNewStreamFilter(`{job="foobar",instance="host-1:234"}`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		minTimestamp := baseTimestamp + (rowsPerBlock-2)*1e9
		maxTimestamp := baseTimestamp + (rowsPerBlock-1)*1e9 - 1
		f := getBaseFilter(minTimestamp, maxTimestamp, sf)
		so := &genericSearchOptions{
			tenantIDs:         []TenantID{tenantID},
			filter:            f,
			resultColumnNames: []string{"_msg"},
		}
		var rowsCount atomic.Uint32
		processBlock := func(workerID uint, br *blockResult) {
			rowsCount.Add(uint32(br.RowsCount()))
		}
		s.search(workersCount, so, nil, processBlock)

		expectedRowsCount := blocksPerStream
		if n := rowsCount.Load(); n != uint32(expectedRowsCount) {
			t.Fatalf("unexpected number of rows; got %d; want %d", n, expectedRowsCount)
		}
	})
	t.Run("matching-stream-id-missing-time-range", func(t *testing.T) {
		sf := mustNewStreamFilter(`{job="foobar",instance="host-1:234"}`)
		tenantID := TenantID{
			AccountID: 1,
			ProjectID: 11,
		}
		minTimestamp := baseTimestamp + (rowsPerBlock+1)*1e9
		maxTimestamp := baseTimestamp + (rowsPerBlock+2)*1e9
		f := getBaseFilter(minTimestamp, maxTimestamp, sf)
		so := &genericSearchOptions{
			tenantIDs:         []TenantID{tenantID},
			filter:            f,
			resultColumnNames: []string{"_msg"},
		}
		processBlock := func(workerID uint, br *blockResult) {
			panic(fmt.Errorf("unexpected match"))
		}
		s.search(workersCount, so, nil, processBlock)
	})

	s.MustClose()
	fs.MustRemoveAll(path)
}

func mustNewStreamFilter(s string) *StreamFilter {
	sf, err := newStreamFilter(s)
	if err != nil {
		panic(fmt.Errorf("unexpected error in newStreamFilter(%q): %w", s, err))
	}
	return sf
}
