package metricsmetadata

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var rowCmpOpts = cmpopts.IgnoreFields(Row{}, "lastWriteTime", "heapIdx")

func TestStorageWrite(t *testing.T) {
	s := NewStorage(4096)
	defer s.MustClose()

	f := func(toIngest []Row, expected []*Row) {
		t.Helper()
		s.Add(toIngest)
		// replace row values with dummy data
		// in order to check possible memory corruption
		dummyValue := []byte(`redacted`)
		for _, row := range toIngest {
			row.Help = append(row.Help[:0], dummyValue...)
			row.MetricFamilyName = append(row.MetricFamilyName[:0], dummyValue...)
			row.Unit = append(row.Unit[:0], dummyValue...)
			row.Type = 0
		}
		got := s.Get(0, "")
		sortRows(expected)
		if diff := cmp.Diff(got, expected, rowCmpOpts); len(diff) > 0 {
			t.Errorf("unexpected rows (-want, +got):\n%s", diff)
		}
	}

	rows := []Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Unit:             []byte("seconds"),
			Help:             []byte("help1"),
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             2,
			Unit:             []byte("bytes"),
			Help:             []byte("help2"),
			AccountID:        1,
			ProjectID:        1,
		},
	}
	expected := []*Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Unit:             []byte("seconds"),
			Help:             []byte("help1"),
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             2,
			Unit:             []byte("bytes"),
			Help:             []byte("help2"),
			AccountID:        1,
			ProjectID:        1,
		},
	}

	f(rows, expected)

	// update Help
	rowToUpdate := []Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Unit:             []byte("seconds"),
			Help:             []byte("UseLessHelp2"),
			AccountID:        1,
			ProjectID:        1,
		},
	}
	expected = []*Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Unit:             []byte("seconds"),
			Help:             []byte("UseLessHelp2"),
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             2,
			Unit:             []byte("bytes"),
			Help:             []byte("help2"),
			AccountID:        1,
			ProjectID:        1,
		},
	}
	f(rowToUpdate, expected)

	// update Unit and Type
	rowToUpdate = []Row{
		{
			MetricFamilyName: []byte("metric2"),
			Type:             5,
			Unit:             []byte("meters"),
			Help:             []byte("help2"),
			AccountID:        1,
			ProjectID:        1,
		},
	}
	expected = []*Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Unit:             []byte("seconds"),
			Help:             []byte("UseLessHelp2"),
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             5,
			Unit:             []byte("meters"),
			Help:             []byte("help2"),
			AccountID:        1,
			ProjectID:        1,
		},
	}
	f(rowToUpdate, expected)

	// add the same metric name to other tenants
	rowToAdd := []Row{
		{
			MetricFamilyName: []byte("metric2"),
			Type:             5,
			Unit:             []byte("meters"),
			Help:             []byte("help2"),
			AccountID:        15,
			ProjectID:        0,
		},
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Unit:             []byte("seconds"),
			Help:             []byte("UseLessHelp2"),
			AccountID:        0,
			ProjectID:        0,
		},
	}

	expected = []*Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Unit:             []byte("seconds"),
			Help:             []byte("UseLessHelp2"),
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Unit:             []byte("seconds"),
			Help:             []byte("UseLessHelp2"),
			AccountID:        0,
			ProjectID:        0,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             5,
			Unit:             []byte("meters"),
			Help:             []byte("help2"),
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             5,
			Unit:             []byte("meters"),
			Help:             []byte("help2"),
			AccountID:        15,
			ProjectID:        0,
		},
	}

	f(rowToAdd, expected)
}

func TestStorageRead(t *testing.T) {
	s := NewStorage(4096)
	defer s.MustClose()

	// Add test data
	rows := []Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Help:             []byte("uselesshelp1"),
			Unit:             []byte("seconds1"),
			AccountID:        0,
			ProjectID:        0,
		},
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Help:             []byte("uselesshelp1"),
			Unit:             []byte("seconds1"),
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             2,
			Help:             []byte("uselesshelp2"),
			Unit:             []byte("seconds2"),
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             2,
			Help:             []byte("uselesshelp2"),
			Unit:             []byte("seconds2"),
			AccountID:        3,
			ProjectID:        15,
		},
		{
			MetricFamilyName: []byte("metric3"),
			Unit:             []byte("unknown"),
			Help:             []byte("help3"),
			Type:             1,
			AccountID:        2,
			ProjectID:        1,
		},
	}
	s.Add(rows)

	f := func(get func() []*Row, expected []*Row) {
		t.Helper()
		got := get()
		sortRows(expected)
		if diff := cmp.Diff(got, expected, rowCmpOpts); len(diff) > 0 {
			t.Errorf("unexpected rows get result (-want, +got):\n%s", diff)
		}
	}

	get := func() []*Row {
		return s.Get(0, "")
	}
	expected := []*Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Help:             []byte("uselesshelp1"),
			Unit:             []byte("seconds1"),
			AccountID:        0,
			ProjectID:        0,
		},
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Help:             []byte("uselesshelp1"),
			Unit:             []byte("seconds1"),
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             2,
			Help:             []byte("uselesshelp2"),
			Unit:             []byte("seconds2"),
			AccountID:        1,
			ProjectID:        1,
		},
		{
			MetricFamilyName: []byte("metric2"),
			Type:             2,
			Help:             []byte("uselesshelp2"),
			Unit:             []byte("seconds2"),
			AccountID:        3,
			ProjectID:        15,
		},
		{
			MetricFamilyName: []byte("metric3"),
			Unit:             []byte("unknown"),
			Help:             []byte("help3"),
			Type:             1,
			AccountID:        2,
			ProjectID:        1,
		},
	}
	f(get, expected)

	// with metric name
	get = func() []*Row {
		return s.Get(0, "metric3")
	}
	expected = []*Row{
		{
			MetricFamilyName: []byte("metric3"),
			Unit:             []byte("unknown"),
			Help:             []byte("help3"),
			Type:             1,
			AccountID:        2,
			ProjectID:        1,
		},
	}
	f(get, expected)

	// with metric name different tenant
	get = func() []*Row {
		return s.Get(0, "metric1")
	}
	expected = []*Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Help:             []byte("uselesshelp1"),
			Unit:             []byte("seconds1"),
			AccountID:        0,
			ProjectID:        0,
		},
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Help:             []byte("uselesshelp1"),
			Unit:             []byte("seconds1"),
			AccountID:        1,
			ProjectID:        1,
		},
	}
	f(get, expected)

	// nonexistent metric name

	get = func() []*Row {
		return s.Get(0, "nonexistent")
	}
	expected = nil
	f(get, expected)

	// with limit
	get = func() []*Row {
		return s.Get(1, "")
	}
	expected = []*Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Help:             []byte("uselesshelp1"),
			Unit:             []byte("seconds1"),
			AccountID:        0,
			ProjectID:        0,
		},
	}
	f(get, expected)

	// for specific tenant
	get = func() []*Row {
		return s.GetForTenant(2, 1, 0, "")
	}
	expected = []*Row{
		{
			MetricFamilyName: []byte("metric3"),
			Unit:             []byte("unknown"),
			Help:             []byte("help3"),
			Type:             1,
			AccountID:        2,
			ProjectID:        1,
		},
	}
	f(get, expected)

	// metric name at tenant
	get = func() []*Row {
		return s.GetForTenant(0, 0, 0, "metric1")
	}
	expected = []*Row{
		{
			MetricFamilyName: []byte("metric1"),
			Type:             1,
			Help:             []byte("uselesshelp1"),
			Unit:             []byte("seconds1"),
			AccountID:        0,
			ProjectID:        0,
		},
	}
	f(get, expected)
}
