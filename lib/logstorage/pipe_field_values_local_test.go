package logstorage

import (
	"reflect"
	"testing"
)

func TestMergeValuesWithHits(t *testing.T) {
	f := func(a [][]ValueWithHits, limit uint64, resetHitsOnLimitExceeded bool, resultExpected []ValueWithHits) {
		t.Helper()

		result := MergeValuesWithHits(a, limit, resetHitsOnLimitExceeded)
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", result, resultExpected)
		}
	}

	var a [][]ValueWithHits
	var resultExpected []ValueWithHits

	// nil input
	a = nil
	resultExpected = []ValueWithHits{}
	f(a, 0, false, resultExpected)

	// no limit
	a = [][]ValueWithHits{
		{
			{
				Value: "foo",
				Hits:  123,
			},
			{
				Value: "bar",
				Hits:  32,
			},
		},
		{
			{
				Value: "bar",
				Hits:  456,
			},
		},
	}
	resultExpected = []ValueWithHits{
		{
			Value: "bar",
			Hits:  488,
		},
		{
			Value: "foo",
			Hits:  123,
		},
	}
	f(a, 0, false, resultExpected)
	f(a, 0, true, resultExpected)

	// no limit, zero hits
	a = [][]ValueWithHits{
		{
			{
				Value: "foo",
				Hits:  123,
			},
			{
				Value: "bar",
				Hits:  0,
			},
		},
		{
			{
				Value: "bar",
				Hits:  13,
			},
		},
	}
	resultExpected = []ValueWithHits{
		{
			Value: "bar",
			Hits:  0,
		},
		{
			Value: "foo",
			Hits:  0,
		},
	}
	f(a, 0, false, resultExpected)
	f(a, 0, true, resultExpected)

	// limit exceeded, no hits reset
	a = [][]ValueWithHits{
		{
			{
				Value: "bar",
				Hits:  123,
			},
		},
		{
			{
				Value: "foo",
				Hits:  33,
			},
			{
				Value: "bar",
				Hits:  365,
			},
		},
	}
	resultExpected = []ValueWithHits{
		{
			Value: "bar",
			Hits:  488,
		},
	}
	f(a, 1, false, resultExpected)

	// limit exceeded, hits reset
	a = [][]ValueWithHits{
		{
			{
				Value: "bar",
				Hits:  123,
			},
		},
		{
			{
				Value: "foo",
				Hits:  33,
			},
			{
				Value: "bar",
				Hits:  365,
			},
		},
	}
	resultExpected = []ValueWithHits{
		{
			Value: "bar",
			Hits:  0,
		},
	}
	f(a, 1, true, resultExpected)
}
