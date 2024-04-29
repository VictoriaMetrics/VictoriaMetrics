package logstorage

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestComplexFilters(t *testing.T) {
	columns := []column{
		{
			name: "foo",
			values: []string{
				"a foo",
				"a foobar",
				"aa abc a",
				"ca afdf a,foobar baz",
				"a fddf foobarbaz",
				"a",
				"a foobar abcdef",
				"a kjlkjf dfff",
				"a ТЕСТЙЦУК НГКШ ",
				"a !!,23.(!1)",
			},
		},
	}

	// (foobar AND NOT baz AND (abcdef OR xyz))
	f := &filterAnd{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "foobar",
			},
			&filterNot{
				f: &filterPhrase{
					fieldName: "foo",
					phrase:    "baz",
				},
			},
			&filterOr{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "abcdef",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "xyz",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, f, "foo", []int{6})

	// (foobaz AND NOT baz AND (abcdef OR xyz))
	f = &filterAnd{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "foobaz",
			},
			&filterNot{
				f: &filterPhrase{
					fieldName: "foo",
					phrase:    "baz",
				},
			},
			&filterOr{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "abcdef",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "xyz",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, f, "foo", nil)

	// (foobaz AND NOT baz AND (abcdef OR xyz OR a))
	f = &filterAnd{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "foobar",
			},
			&filterNot{
				f: &filterPhrase{
					fieldName: "foo",
					phrase:    "baz",
				},
			},
			&filterOr{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "abcdef",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "xyz",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "a",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, f, "foo", []int{1, 6})

	// (foobaz AND NOT qwert AND (abcdef OR xyz OR a))
	f = &filterAnd{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "foobar",
			},
			&filterNot{
				f: &filterPhrase{
					fieldName: "foo",
					phrase:    "qwert",
				},
			},
			&filterOr{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "abcdef",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "xyz",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "a",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, f, "foo", []int{1, 3, 6})
}

func TestStreamFilter(t *testing.T) {
	columns := []column{
		{
			name: "foo",
			values: []string{
				"a foo",
				"a foobar",
				"aa abc a",
				"ca afdf a,foobar baz",
				"a fddf foobarbaz",
				"",
				"a foobar",
				"a kjlkjf dfff",
				"a ТЕСТЙЦУК НГКШ ",
				"a !!,23.(!1)",
			},
		},
	}

	// Match
	f := &filterExact{
		fieldName: "job",
		value:     "foobar",
	}
	testFilterMatchForColumns(t, columns, f, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	// Mismatch
	f = &filterExact{
		fieldName: "job",
		value:     "abc",
	}
	testFilterMatchForColumns(t, columns, f, "foo", nil)
}

func testFilterMatchForTimestamps(t *testing.T, timestamps []int64, f filter, expectedRowIdxs []int) {
	t.Helper()

	// Create the test storage
	const storagePath = "testFilterMatchForTimestamps"
	cfg := &StorageConfig{}
	s := MustOpenStorage(storagePath, cfg)

	// Generate rows
	getValue := func(rowIdx int) string {
		return fmt.Sprintf("some value for row %d", rowIdx)
	}
	tenantID := TenantID{
		AccountID: 123,
		ProjectID: 456,
	}
	generateRowsFromTimestamps(s, tenantID, timestamps, getValue)

	expectedResults := make([]string, len(expectedRowIdxs))
	expectedTimestamps := make([]int64, len(expectedRowIdxs))
	for i, idx := range expectedRowIdxs {
		expectedResults[i] = getValue(idx)
		expectedTimestamps[i] = timestamps[idx]
	}

	testFilterMatchForStorage(t, s, tenantID, f, "_msg", expectedResults, expectedTimestamps)

	// Close and delete the test storage
	s.MustClose()
	fs.MustRemoveAll(storagePath)
}

func testFilterMatchForColumns(t *testing.T, columns []column, f filter, resultColumnName string, expectedRowIdxs []int) {
	t.Helper()

	// Create the test storage
	const storagePath = "testFilterMatchForColumns"
	cfg := &StorageConfig{}
	s := MustOpenStorage(storagePath, cfg)

	// Generate rows
	tenantID := TenantID{
		AccountID: 123,
		ProjectID: 456,
	}
	generateRowsFromColumns(s, tenantID, columns)

	var values []string
	for _, c := range columns {
		if c.name == resultColumnName {
			values = c.values
			break
		}
	}
	expectedResults := make([]string, len(expectedRowIdxs))
	expectedTimestamps := make([]int64, len(expectedRowIdxs))
	for i, idx := range expectedRowIdxs {
		expectedResults[i] = values[idx]
		expectedTimestamps[i] = int64(idx) * 1e9
	}

	testFilterMatchForStorage(t, s, tenantID, f, resultColumnName, expectedResults, expectedTimestamps)

	// Close and delete the test storage
	s.MustClose()
	fs.MustRemoveAll(storagePath)
}

func testFilterMatchForStorage(t *testing.T, s *Storage, tenantID TenantID, f filter, resultColumnName string, expectedResults []string, expectedTimestamps []int64) {
	t.Helper()

	so := &genericSearchOptions{
		tenantIDs:         []TenantID{tenantID},
		filter:            f,
		resultColumnNames: []string{resultColumnName},
	}
	workersCount := 3
	s.search(workersCount, so, nil, func(_ uint, br *blockResult) {
		// Verify tenantID
		if !br.streamID.tenantID.equal(&tenantID) {
			t.Fatalf("unexpected tenantID in blockResult; got %s; want %s", &br.streamID.tenantID, &tenantID)
		}

		// Verify columns
		if len(br.cs) != 1 {
			t.Fatalf("unexpected number of columns in blockResult; got %d; want 1", len(br.cs))
		}
		results := br.getColumnValues(0)
		if !reflect.DeepEqual(results, expectedResults) {
			t.Fatalf("unexpected results matched;\ngot\n%q\nwant\n%q", results, expectedResults)
		}

		// Verify timestamps
		if br.timestamps == nil {
			br.timestamps = []int64{}
		}
		if !reflect.DeepEqual(br.timestamps, expectedTimestamps) {
			t.Fatalf("unexpected timestamps;\ngot\n%d\nwant\n%d", br.timestamps, expectedTimestamps)
		}
	})
}

func generateRowsFromColumns(s *Storage, tenantID TenantID, columns []column) {
	streamTags := []string{
		"job",
		"instance",
	}
	lr := GetLogRows(streamTags, nil)
	var fields []Field
	for i := range columns[0].values {
		// Add stream tags
		fields = append(fields[:0], Field{
			Name:  "job",
			Value: "foobar",
		}, Field{
			Name:  "instance",
			Value: "host1:234",
		})
		// Add other columns
		for j := range columns {
			fields = append(fields, Field{
				Name:  columns[j].name,
				Value: columns[j].values[i],
			})
		}
		timestamp := int64(i) * 1e9
		lr.MustAdd(tenantID, timestamp, fields)
	}
	s.MustAddRows(lr)
	PutLogRows(lr)
}

func generateRowsFromTimestamps(s *Storage, tenantID TenantID, timestamps []int64, getValue func(rowIdx int) string) {
	lr := GetLogRows(nil, nil)
	var fields []Field
	for i, timestamp := range timestamps {
		fields = append(fields[:0], Field{
			Name:  "_msg",
			Value: getValue(i),
		})
		lr.MustAdd(tenantID, timestamp, fields)
	}
	s.MustAddRows(lr)
	PutLogRows(lr)
}
