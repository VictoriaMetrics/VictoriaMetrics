package vmimport

import (
	"math"
	"reflect"
	"testing"
)

func TestRowsUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var rows Rows
		rows.Unmarshal(s)
		if len(rows.Rows) != 0 {
			t.Fatalf("expecting zero rows; got %d rows", len(rows.Rows))
		}

		// Try again
		rows.Unmarshal(s)
		if len(rows.Rows) != 0 {
			t.Fatalf("expecting zero rows; got %d rows", len(rows.Rows))
		}
	}

	// Invalid json line
	f("")
	f("\n")
	f("foo\n")
	f("123")
	f("[1,3]")
	f("{}")
	f("[]")
	f(`{"foo":"bar"}`)

	// Invalid metric
	f(`{"metric":123,"values":[1,2],"timestamps":[3,4]}`)
	f(`{"metric":[123],"values":[1,2],"timestamps":[3,4]}`)
	f(`{"metric":[],"values":[1,2],"timestamps":[3,4]}`)
	f(`{"metric":{},"values":[1,2],"timestamps":[3,4]}`)
	f(`{"metric":null,"values":[1,2],"timestamps":[3,4]}`)
	f(`{"values":[1,2],"timestamps":[3,4]}`)

	// Invalid values
	f(`{"metric":{"foo":"bar"},"values":1,"timestamps":[3,4]}`)
	f(`{"metric":{"foo":"bar"},"values":{"x":1},"timestamps":[3,4]}`)
	f(`{"metric":{"foo":"bar"},"values":{"x":1},"timestamps":[3,4]}`)
	f(`{"metric":{"foo":"bar"},"values":null,"timestamps":[3,4]}`)
	f(`{"metric":{"foo":"bar"},"timestamps":[3,4]}`)

	// Invalid timestamps
	f(`{"metric":{"foo":"bar"},"values":[1,2],"timestamps":3}`)
	f(`{"metric":{"foo":"bar"},"values":[1,2],"timestamps":false}`)
	f(`{"metric":{"foo":"bar"},"values":[1,2],"timestamps":{}}`)
	f(`{"metric":{"foo":"bar"},"values":[1,2]}`)

	// values and timestamps count mismatch
	f(`{"metric":{"foo":"bar"},"values":[],"timestamps":[]}`)
	f(`{"metric":{"foo":"bar"},"values":[],"timestamps":[1]}`)
	f(`{"metric":{"foo":"bar"},"values":[2],"timestamps":[]}`)
	f(`{"metric":{"foo":"bar"},"values":[2],"timestamps":[3,4]}`)
	f(`{"metric":{"foo":"bar"},"values":[2,3],"timestamps":[4]}`)

	// Garbage after the line
	f(`{"metric":{"foo":"bar"},"values":[2],"timestamps":[4]}{}`)
}

func TestRowsUnmarshalSuccess(t *testing.T) {
	f := func(s string, rowsExpected *Rows) {
		t.Helper()
		var rows Rows
		rows.Unmarshal(s)
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		// Try unmarshaling again
		rows.Unmarshal(s)
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		rows.Reset()
		if len(rows.Rows) != 0 {
			t.Fatalf("non-empty rows after reset: %+v", rows.Rows)
		}
	}

	// Empty line
	f("", &Rows{})
	f("\n\n", &Rows{})
	f("\n\r\n", &Rows{})

	// Single line with a single tag
	f(`{"metric":{"foo":"bar"},"values":[1.23],"timestamps":[456]}`, &Rows{
		Rows: []Row{{
			Tags: []Tag{{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			}},
			Values:     []float64{1.23},
			Timestamps: []int64{456},
		}},
	})

	// Inf and nan values
	f(`{"metric":{"foo":"bar"},"values":[Inf, -Inf],"timestamps":[456, 789]}`, &Rows{
		Rows: []Row{{
			Tags: []Tag{{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			}},
			Values:     []float64{math.Inf(1), math.Inf(-1)},
			Timestamps: []int64{456, 789},
		}},
	})

	// Line with multiple tags
	f(`{"metric":{"foo":"bar","baz":"xx"},"values":[1.23, -3.21],"timestamps" : [456,789]}`, &Rows{
		Rows: []Row{{
			Tags: []Tag{
				{
					Key:   []byte("foo"),
					Value: []byte("bar"),
				},
				{
					Key:   []byte("baz"),
					Value: []byte("xx"),
				},
			},
			Values:     []float64{1.23, -3.21},
			Timestamps: []int64{456, 789},
		}},
	})

	// Multiple lines
	f(`{"metric":{"foo":"bar","baz":"xx"},"values":[1.23, -3.21],"timestamps" : [456,789]}
{"metric":{"__name__":"xx"},"values":[34],"timestamps" : [11]}
`, &Rows{
		Rows: []Row{
			{
				Tags: []Tag{
					{
						Key:   []byte("foo"),
						Value: []byte("bar"),
					},
					{
						Key:   []byte("baz"),
						Value: []byte("xx"),
					},
				},
				Values:     []float64{1.23, -3.21},
				Timestamps: []int64{456, 789},
			},
			{
				Tags: []Tag{
					{
						Key:   []byte("__name__"),
						Value: []byte("xx"),
					},
				},
				Values:     []float64{34},
				Timestamps: []int64{11},
			},
		},
	})

	// Multiple lines with invalid line in the middle.
	f(`{"metric":{"xfoo":"bar","baz":"xx"},"values":[1.232, -3.21],"timestamps" : [456,7890]}
garbage here
{"metric":{"__name__":"xxy"},"values":[34],"timestamps" : [111]}`, &Rows{
		Rows: []Row{
			{
				Tags: []Tag{
					{
						Key:   []byte("xfoo"),
						Value: []byte("bar"),
					},
					{
						Key:   []byte("baz"),
						Value: []byte("xx"),
					},
				},
				Values:     []float64{1.232, -3.21},
				Timestamps: []int64{456, 7890},
			},
			{
				Tags: []Tag{
					{
						Key:   []byte("__name__"),
						Value: []byte("xxy"),
					},
				},
				Values:     []float64{34},
				Timestamps: []int64{111},
			},
		},
	})

	// No newline after the second line.
	f(`{"metric":{"foo":"bar","baz":"xx"},"values":[1.23, -3.21],"timestamps" : [456,789]}
{"metric":{"__name__":"xx"},"values":[34],"timestamps" : [11]}`, &Rows{
		Rows: []Row{
			{
				Tags: []Tag{
					{
						Key:   []byte("foo"),
						Value: []byte("bar"),
					},
					{
						Key:   []byte("baz"),
						Value: []byte("xx"),
					},
				},
				Values:     []float64{1.23, -3.21},
				Timestamps: []int64{456, 789},
			},
			{
				Tags: []Tag{
					{
						Key:   []byte("__name__"),
						Value: []byte("xx"),
					},
				},
				Values:     []float64{34},
				Timestamps: []int64{11},
			},
		},
	})
}
