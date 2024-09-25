package logstorage

import (
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

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

func testFilterMatchForColumns(t *testing.T, columns []column, f filter, neededColumnName string, expectedRowIdxs []int) {
	t.Helper()

	// Create the test storage
	storagePath := t.Name()
	cfg := &StorageConfig{
		Retention: time.Duration(100 * 365 * nsecsPerDay),
	}
	s := MustOpenStorage(storagePath, cfg)

	// Generate rows
	tenantID := TenantID{
		AccountID: 123,
		ProjectID: 456,
	}
	generateRowsFromColumns(s, tenantID, columns)

	var values []string
	for _, c := range columns {
		if c.name == neededColumnName {
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

	testFilterMatchForStorage(t, s, tenantID, f, neededColumnName, expectedResults, expectedTimestamps)

	// Close and delete the test storage
	s.MustClose()
	fs.MustRemoveAll(storagePath)
}

func testFilterMatchForStorage(t *testing.T, s *Storage, tenantID TenantID, f filter, neededColumnName string, expectedValues []string, expectedTimestamps []int64) {
	t.Helper()

	so := newTestGenericSearchOptions([]TenantID{tenantID}, f, []string{neededColumnName, "_time"})

	type result struct {
		value     string
		timestamp int64
	}
	var resultsMu sync.Mutex
	var results []result

	const workersCount = 3
	s.search(workersCount, so, nil, func(_ uint, br *blockResult) {
		// Verify columns
		cs := br.getColumns()
		if len(cs) != 2 {
			t.Fatalf("unexpected number of columns in blockResult; got %d; want 2", len(cs))
		}
		values := cs[0].getValues(br)
		timestamps := br.getTimestamps()
		resultsMu.Lock()
		for i, v := range values {
			results = append(results, result{
				value:     strings.Clone(v),
				timestamp: timestamps[i],
			})
		}
		resultsMu.Unlock()
	})

	sort.Slice(results, func(i, j int) bool {
		return results[i].timestamp < results[j].timestamp
	})

	timestamps := make([]int64, len(results))
	values := make([]string, len(results))

	for i, r := range results {
		timestamps[i] = r.timestamp
		values[i] = r.value
	}

	if !reflect.DeepEqual(timestamps, expectedTimestamps) {
		t.Fatalf("unexpected timestamps;\ngot\n%d\nwant\n%d", timestamps, expectedTimestamps)
	}
	if !reflect.DeepEqual(values, expectedValues) {
		t.Fatalf("unexpected values;\ngot\n%q\nwant\n%q", values, expectedValues)
	}
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
