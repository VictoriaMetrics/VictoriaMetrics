package vmimport

import (
	"fmt"
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
	f(`{"metric":{"foo":"bar"},"values":"NaN","timestamps":[3,4]}`)
	f(`{"metric":{"foo":"bar"},"values":[["NaN"]],"timestamps":[3,4]}`)

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

		if err := compareRows(&rows, rowsExpected); err != nil {
			t.Fatalf("unexpected rows: %s;\ngot\n%+v;\nwant\n%+v", err, rows.Rows, rowsExpected.Rows)
		}

		// Try unmarshaling again
		rows.Unmarshal(s)
		if err := compareRows(&rows, rowsExpected); err != nil {
			t.Fatalf("unexpected rows at second unmarshal: %s;\ngot\n%+v;\nwant\n%+v", err, rows.Rows, rowsExpected.Rows)
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
	f(`{"metric":{"foo":"bar"},"values":[Inf, -Inf, "Infinity", "-Infinity", NaN, "NaN", null, "null", 1.2],"timestamps":[456, 789, 123, 0, 1, 42, 2, 3, 7]}`, &Rows{
		Rows: []Row{{
			Tags: []Tag{{
				Key:   []byte("foo"),
				Value: []byte("bar"),
			}},
			Values:     []float64{inf, -inf, inf, -inf, nan, nan, nan, nan, 1.2},
			Timestamps: []int64{456, 789, 123, 0, 1, 42, 2, 3, 7},
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

func compareRows(rows, rowsExpected *Rows) error {
	if len(rows.Rows) != len(rowsExpected.Rows) {
		return fmt.Errorf("unexpected number of rows; got %d; want %d", len(rows.Rows), len(rowsExpected.Rows))
	}
	for i, row := range rows.Rows {
		rowExpected := rowsExpected.Rows[i]
		if err := compareSingleRow(&row, &rowExpected); err != nil {
			return fmt.Errorf("unexpected row at position #%d: %w", i, err)
		}
	}
	return nil
}

func compareSingleRow(row, rowExpected *Row) error {
	if !reflect.DeepEqual(row.Tags, rowExpected.Tags) {
		return fmt.Errorf("unexpected tags; got %q; want %q", row.Tags, rowExpected.Tags)
	}
	if !reflect.DeepEqual(row.Timestamps, rowExpected.Timestamps) {
		return fmt.Errorf("unexpected timestamps; got %d; want %d", row.Timestamps, rowExpected.Timestamps)
	}
	if err := compareValues(row.Values, rowExpected.Values); err != nil {
		return fmt.Errorf("unexpected values; got %v; want %v", row.Values, rowExpected.Values)
	}
	return nil
}

func compareValues(values, valuesExpected []float64) error {
	if len(values) != len(valuesExpected) {
		return fmt.Errorf("unexpected number of values; got %d; want %d", len(values), len(valuesExpected))
	}
	for i, v := range values {
		vExpected := valuesExpected[i]
		if math.IsNaN(v) {
			if !math.IsNaN(vExpected) {
				return fmt.Errorf("expecting NaN at position #%d; got %v", i, v)
			}
		} else if v != vExpected {
			return fmt.Errorf("unepxected value at position #%d; got %v; want %v", i, v, vExpected)
		}
	}
	return nil
}
