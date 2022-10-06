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
	f(`{"metric":{"foo":"bar"},"values":["foo"],"timestamps":[3]}`)
	f(`{"metric":{"foo":"bar"},"values":null,"timestamps":[3,4]}`)
	f(`{"metric":{"foo":"bar"},"values":"null","timestamps":[3,4]}`)
	f(`{"metric":{"foo":"bar"},"values":["NaN"],"timestamps":[3,4]}`)

	// Invalid timestamps
	f(`{"metric":{"foo":"bar"},"values":[1,2],"timestamps":3}`)
	f(`{"metric":{"foo":"bar"},"values":[1,2],"timestamps":false}`)
	f(`{"metric":{"foo":"bar"},"values":[1,2],"timestamps":{}}`)
	f(`{"metric":{"foo":"bar"},"values":[1,2]}`)
	f(`{"metric":{"foo":"bar"},"values":[1,2],"timestamps":[1,"foo"]}`)

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

		if containsNaN(rows) {
			if !checkNaN(rows, rowsExpected) {
				t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
				return
			}
			return
		}

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

	// Inf and nan, null values
	f(`{"metric":{"foo":"bar"},"values":[Inf, -Inf, "Infinity", "-Infinity", NaN, null, "null"],"timestamps":[456, 789, 123, 0, 1, 2, 3]}`, &Rows{
		Rows: []Row{{
			Tags: []Tag{{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			}},
			Values:     []float64{math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1), math.NaN(), math.NaN(), math.NaN()},
			Timestamps: []int64{456, 789, 123, 0, 1, 2, 3},
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

func Test_getFloat64FromStringValue(t *testing.T) {
	f := func(name, strVal string, want float64, wantErr bool) {
		t.Run(name, func(t *testing.T) {
			got, err := getSpecialFloat64ValueFromString(strVal)
			if (err != nil) != wantErr {
				t.Errorf("getSpecialFloat64ValueFromString() error = %v, wantErr %v", err, wantErr)
				return
			}

			if math.IsNaN(want) {
				if !math.IsNaN(got) {
					t.Fatalf("unexpected result; got %v; want %v", got, want)
					return
				}
				return
			}

			if got != want {
				t.Errorf("getSpecialFloat64ValueFromString() got = %v, want %v", got, want)
			}
		})
	}

	f("empty string", "", 0, true)
	f("unsupported string", "1", 0, true)
	f("null string", "null", 0, true)
	f("infinity string", "\"Infinity\"", math.Inf(1), false)
	f("-infinity string", "\"-Infinity\"", math.Inf(-1), false)
	f("null string", "\"null\"", math.NaN(), false)
}

func containsNaN(rows Rows) bool {
	for _, row := range rows.Rows {
		for _, f := range row.Values {
			if math.IsNaN(f) {
				return true
			}
		}
	}
	return false
}

func checkNaN(rows Rows, expectedRows *Rows) bool {
	for i, row := range rows.Rows {
		r := expectedRows.Rows[i]
		for j, f := range row.Values {
			if math.IsNaN(f) && math.IsNaN(r.Values[j]) {
				return true
			}
		}
	}
	return false
}
