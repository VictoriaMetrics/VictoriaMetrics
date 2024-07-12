package stepper

import (
	"reflect"
	"testing"
	"time"
)

type testTimeRange []string

func mustParseDatetime(t string) time.Time {
	result, err := time.Parse(time.RFC3339, t)
	if err != nil {
		panic(err)
	}
	return result
}

func TestSplitDateRange_Failure(t *testing.T) {
	f := func(startStr, endStr, granularity string) {
		t.Helper()

		start := mustParseDatetime(startStr)
		end := mustParseDatetime(endStr)

		_, err := SplitDateRange(start, end, granularity, false)
		if err == nil {
			t.Fatalf("expecting non-nil result")
		}
	}

	// validates start is before end
	f("2022-02-01T00:00:00Z", "2022-01-01T00:00:00Z", StepMonth)

	// validates granularity value
	f("2022-01-01T00:00:00Z", "2022-02-01T00:00:00Z", "non-existent-format")
}

func TestSplitDateRange_Success(t *testing.T) {
	f := func(startStr, endStr, granularity string, resultExpected []testTimeRange) {
		t.Helper()

		start := mustParseDatetime(startStr)
		end := mustParseDatetime(endStr)

		result, err := SplitDateRange(start, end, granularity, false)
		if err != nil {
			t.Fatalf("SplitDateRange() error: %s", err)
		}

		var testExpectedResults [][]time.Time
		for _, dr := range resultExpected {
			testExpectedResults = append(testExpectedResults, []time.Time{
				mustParseDatetime(dr[0]),
				mustParseDatetime(dr[1]),
			})
		}

		if !reflect.DeepEqual(result, testExpectedResults) {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", result, testExpectedResults)
		}
	}

	// month chunking
	f("2022-01-03T11:11:11Z", "2022-03-03T12:12:12Z", StepMonth, []testTimeRange{
		{
			"2022-01-03T11:11:11Z",
			"2022-01-31T23:59:59.999999999Z",
		},
		{
			"2022-02-01T00:00:00Z",
			"2022-02-28T23:59:59.999999999Z",
		},
		{
			"2022-03-01T00:00:00Z",
			"2022-03-03T12:12:12Z",
		},
	})

	// daily chunking
	f("2022-01-03T11:11:11Z", "2022-01-05T12:12:12Z", StepDay, []testTimeRange{
		{
			"2022-01-03T11:11:11Z",
			"2022-01-04T11:11:11Z",
		},
		{
			"2022-01-04T11:11:11Z",
			"2022-01-05T11:11:11Z",
		},
		{
			"2022-01-05T11:11:11Z",
			"2022-01-05T12:12:12Z",
		},
	})

	// hourly chunking
	f("2022-01-03T11:11:11Z", "2022-01-03T14:14:14Z", StepHour, []testTimeRange{
		{
			"2022-01-03T11:11:11Z",
			"2022-01-03T12:11:11Z",
		},
		{
			"2022-01-03T12:11:11Z",
			"2022-01-03T13:11:11Z",
		},
		{
			"2022-01-03T13:11:11Z",
			"2022-01-03T14:11:11Z",
		},
		{
			"2022-01-03T14:11:11Z",
			"2022-01-03T14:14:14Z",
		},
	})

	// month chunking with one day time range
	f("2022-01-03T11:11:11Z", "2022-01-04T12:12:12Z", StepMonth, []testTimeRange{
		{
			"2022-01-03T11:11:11Z",
			"2022-01-04T12:12:12Z",
		},
	})

	// month chunking with same day time range
	f("2022-01-03T11:11:11Z", "2022-01-03T12:12:12Z", StepMonth, []testTimeRange{
		{
			"2022-01-03T11:11:11Z",
			"2022-01-03T12:12:12Z",
		},
	})

	// month chunking with one month and two days range
	f("2022-01-03T11:11:11Z", "2022-02-03T00:00:00Z", StepMonth, []testTimeRange{
		{
			"2022-01-03T11:11:11Z",
			"2022-01-31T23:59:59.999999999Z",
		},
		{
			"2022-02-01T00:00:00Z",
			"2022-02-03T00:00:00Z",
		},
	})

	// week chunking with not full week
	f("2023-07-30T00:00:00Z", "2023-08-05T23:59:59.999999999Z", StepWeek, []testTimeRange{
		{
			"2023-07-30T00:00:00Z",
			"2023-08-05T23:59:59.999999999Z",
		},
	})

	// week chunking with start of the week and end of the week
	f("2023-07-30T00:00:00Z", "2023-08-06T00:00:00Z", StepWeek, []testTimeRange{
		{
			"2023-07-30T00:00:00Z",
			"2023-08-06T00:00:00Z",
		},
	})

	// week chunking with next one day week
	f("2023-07-30T00:00:00Z", "2023-08-07T01:12:00Z", StepWeek, []testTimeRange{
		{
			"2023-07-30T00:00:00Z",
			"2023-08-06T00:00:00Z",
		},
		{
			"2023-08-06T00:00:00Z",
			"2023-08-07T01:12:00Z",
		},
	})

	// week chunking with month and not full week representation
	f("2023-07-30T00:00:00Z", "2023-09-01T01:12:00Z", StepWeek, []testTimeRange{
		{
			"2023-07-30T00:00:00Z",
			"2023-08-06T00:00:00Z",
		},
		{
			"2023-08-06T00:00:00Z",
			"2023-08-13T00:00:00Z",
		},
		{
			"2023-08-13T00:00:00Z",
			"2023-08-20T00:00:00Z",
		},
		{
			"2023-08-20T00:00:00Z",
			"2023-08-27T00:00:00Z",
		},
		{
			"2023-08-27T00:00:00Z",
			"2023-09-01T01:12:00Z",
		},
	})
}

func TestSplitDateRange_Reverse_Failure(t *testing.T) {
	f := func(startStr, endStr, granularity string) {
		t.Helper()

		start := mustParseDatetime(startStr)
		end := mustParseDatetime(endStr)

		_, err := SplitDateRange(start, end, granularity, true)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// validates start is before end
	f("2022-02-01T00:00:00Z", "2022-01-01T00:00:00Z", StepMonth)

	// validates granularity value
	f("2022-01-01T00:00:00Z", "2022-02-01T00:00:00Z", "non-existent-format")
}

func TestSplitDateRange_Reverse_Success(t *testing.T) {
	f := func(startStr, endStr, granularity string, resultExpected []testTimeRange) {
		t.Helper()

		start := mustParseDatetime(startStr)
		end := mustParseDatetime(endStr)

		result, err := SplitDateRange(start, end, granularity, true)
		if err != nil {
			t.Fatalf("SplitDateRange() error: %s", err)
		}

		var testExpectedResults [][]time.Time
		for _, dr := range resultExpected {
			testExpectedResults = append(testExpectedResults, []time.Time{
				mustParseDatetime(dr[0]),
				mustParseDatetime(dr[1]),
			})
		}

		if !reflect.DeepEqual(result, testExpectedResults) {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%v", result, testExpectedResults)
		}
	}

	// month chunking
	f("2022-01-03T11:11:11Z", "2022-03-03T12:12:12Z", StepMonth, []testTimeRange{
		{
			"2022-03-01T00:00:00Z",
			"2022-03-03T12:12:12Z",
		},
		{
			"2022-02-01T00:00:00Z",
			"2022-02-28T23:59:59.999999999Z",
		},
		{
			"2022-01-03T11:11:11Z",
			"2022-01-31T23:59:59.999999999Z",
		},
	})

	// daily chunking
	f("2022-01-03T11:11:11Z", "2022-01-05T12:12:12Z", StepDay, []testTimeRange{
		{
			"2022-01-05T11:11:11Z",
			"2022-01-05T12:12:12Z",
		},
		{
			"2022-01-04T11:11:11Z",
			"2022-01-05T11:11:11Z",
		},
		{
			"2022-01-03T11:11:11Z",
			"2022-01-04T11:11:11Z",
		},
	})

	// hourly chunking
	f("2022-01-03T11:11:11Z", "2022-01-03T14:14:14Z", StepHour, []testTimeRange{
		{
			"2022-01-03T14:11:11Z",
			"2022-01-03T14:14:14Z",
		},
		{
			"2022-01-03T13:11:11Z",
			"2022-01-03T14:11:11Z",
		},
		{
			"2022-01-03T12:11:11Z",
			"2022-01-03T13:11:11Z",
		},
		{
			"2022-01-03T11:11:11Z",
			"2022-01-03T12:11:11Z",
		},
	})

	// month chunking with one day time range
	f("2022-01-03T11:11:11Z", "2022-01-04T12:12:12Z", StepMonth, []testTimeRange{
		{
			"2022-01-03T11:11:11Z",
			"2022-01-04T12:12:12Z",
		},
	})

	// month chunking with same day time range
	f("2022-01-03T11:11:11Z", "2022-01-03T12:12:12Z", StepMonth, []testTimeRange{
		{
			"2022-01-03T11:11:11Z",
			"2022-01-03T12:12:12Z",
		},
	})

	// month chunking with one month and two days range
	f("2022-01-03T11:11:11Z", "2022-02-03T00:00:00Z", StepMonth, []testTimeRange{
		{
			"2022-02-01T00:00:00Z",
			"2022-02-03T00:00:00Z",
		},
		{
			"2022-01-03T11:11:11Z",
			"2022-01-31T23:59:59.999999999Z",
		},
	})

	// week chunking with not full week
	f("2023-07-30T00:00:00Z", "2023-08-05T23:59:59.999999999Z", StepWeek, []testTimeRange{
		{
			"2023-07-30T00:00:00Z",
			"2023-08-05T23:59:59.999999999Z",
		},
	})

	// week chunking with start of the week and end of the week
	f("2023-07-30T00:00:00Z", "2023-08-06T00:00:00Z", StepWeek, []testTimeRange{
		{
			"2023-07-30T00:00:00Z",
			"2023-08-06T00:00:00Z",
		},
	})

	// week chunking with next one day week
	f("2023-07-30T00:00:00Z", "2023-08-07T01:12:00Z", StepWeek, []testTimeRange{
		{
			"2023-08-06T00:00:00Z",
			"2023-08-07T01:12:00Z",
		},
		{
			"2023-07-30T00:00:00Z",
			"2023-08-06T00:00:00Z",
		},
	})

	// week chunking with month and not full week representation
	f("2023-07-30T00:00:00Z", "2023-09-01T01:12:00Z", StepWeek, []testTimeRange{
		{
			"2023-08-27T00:00:00Z",
			"2023-09-01T01:12:00Z",
		},
		{
			"2023-08-20T00:00:00Z",
			"2023-08-27T00:00:00Z",
		},
		{
			"2023-08-13T00:00:00Z",
			"2023-08-20T00:00:00Z",
		},
		{
			"2023-08-06T00:00:00Z",
			"2023-08-13T00:00:00Z",
		},
		{
			"2023-07-30T00:00:00Z",
			"2023-08-06T00:00:00Z",
		},
	})
}
