package influx

import "testing"

func TestFetchQuery(t *testing.T) {
	f := func(s *Series, timeFilter, resultExpected string) {
		t.Helper()

		result := s.fetchQuery(timeFilter)
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
		LabelPairs: []LabelPair{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
	}, "", `select "value" from "cpu" where "foo"::tag='bar'`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
		LabelPairs: []LabelPair{
			{
				Name:  "foo",
				Value: "bar",
			},
			{
				Name:  "baz",
				Value: "qux",
			},
		},
	}, "", `select "value" from "cpu" where "foo"::tag='bar' and "baz"::tag='qux'`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
		LabelPairs: []LabelPair{
			{
				Name:  "foo",
				Value: "b'ar",
			},
		},
	}, "time >= now()", `select "value" from "cpu" where "foo"::tag='b\'ar' and time >= now()`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
		LabelPairs: []LabelPair{
			{
				Name:  "name",
				Value: `dev-mapper-centos\x2dswap.swap`,
			},
			{
				Name:  "state",
				Value: "dev-mapp'er-c'en'tos",
			},
		},
	}, "time >= now()", `select "value" from "cpu" where "name"::tag='dev-mapper-centos\\x2dswap.swap' and "state"::tag='dev-mapp\'er-c\'en\'tos' and time >= now()`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
	}, "time >= now()", `select "value" from "cpu" where time >= now()`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
	}, "", `select "value" from "cpu"`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value1",
		EmptyTags:   []string{"e1", "e2", "e3"},
	}, "", `select "value1" from "cpu" where "e1"::tag='' and "e2"::tag='' and "e3"::tag=''`)
}

func TestTimeFilter(t *testing.T) {
	f := func(start, end, resultExpected string) {
		t.Helper()

		result := timeFilter(start, end)
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%s", result, resultExpected)
		}
	}

	// no start and end filters
	f("", "", "")

	// missing end filter
	f("2020-01-01T20:07:00Z", "", "time >= '2020-01-01T20:07:00Z'")

	// missing start filter
	f("", "2020-01-01T21:07:00Z", "time <= '2020-01-01T21:07:00Z'")

	// both start and end filters
	f("2020-01-01T20:07:00Z", "2020-01-01T21:07:00Z", "time >= '2020-01-01T20:07:00Z' and time <= '2020-01-01T21:07:00Z'")
}

func TestGetSeriesCommand(t *testing.T) {
	f := func(filterSeries, filterTime, expCommand string) {
		t.Helper()

		c := &Client{
			filterTime:   filterTime,
			filterSeries: filterSeries,
		}
		gotCommand := c.getSeriesCommand()
		if gotCommand != expCommand {
			t.Fatalf("unexpected command\ngot\n%s\nwant\n%s", gotCommand, expCommand)
		}
	}

	f("", "", "show series")
	f("from cpu", "", "show series from cpu")
	f("from cpu where arch='x86'", "", "show series from cpu where arch='x86'")
	f("", "time >= '2020-01-01T20:07:00Z'", "show series where time >= '2020-01-01T20:07:00Z'")
	f("from cpu", "time >= '2020-01-01T20:07:00Z'", "show series from cpu where time >= '2020-01-01T20:07:00Z'")
	f("from cpu where arch='x86'", "time >= '2020-01-01T20:07:00Z'", "show series from cpu where arch='x86' AND time >= '2020-01-01T20:07:00Z'")
	f("from cpu where arch='x86' AND hostname='host_2753'", "time >= '2020-01-01T20:07:00Z'", "show series from cpu where arch='x86' AND hostname='host_2753' AND time >= '2020-01-01T20:07:00Z'")
}
