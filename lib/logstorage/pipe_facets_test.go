package logstorage

import (
	"testing"
)

func TestParsePipeFacetsSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`facets`)
	f(`facets 15`)
	f(`facets 15 max_values_per_field 20`)
	f(`facets max_values_per_field 20`)
	f(`facets max_value_len 123`)
	f(`facets 34 max_values_per_field 20 max_value_len 30`)
}

func TestParsePipeFacetsFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`facets foo`)
	f(`facets 5 foo`)
	f(`facets max_values_per_field`)
	f(`facets 123 max_values_per_field`)
	f(`facets 123 max_values_per_field bar`)
	f(`facets 123 max_value_len`)
	f(`facets 123 max_value_len bar`)
}

func TestPipeFacets(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("facets 1", [][]Field{
		{
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"a", "2"},
			{"b", "3"},
		},
		{
			{"a", `2`},
			{"b", `54`},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"field_name", "a"},
			{"field_value", "2"},
			{"hits", "3"},
		},
		{
			{"field_name", "b"},
			{"field_value", "3"},
			{"hits", "2"},
		},
		{
			{"field_name", "c"},
			{"field_value", "d"},
			{"hits", "1"},
		},
	})

	f("facets", [][]Field{
		{
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"a", "2"},
			{"b", "3"},
		},
		{
			{"a", `2`},
			{"b", `54`},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"field_name", "a"},
			{"field_value", "2"},
			{"hits", "3"},
		},
		{
			{"field_name", "b"},
			{"field_value", "3"},
			{"hits", "2"},
		},
		{
			{"field_name", "b"},
			{"field_value", "54"},
			{"hits", "1"},
		},
		{
			{"field_name", "c"},
			{"field_value", "d"},
			{"hits", "1"},
		},
	})
}

func TestPipeFacetsUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("facets", "*", "", "*", "")

	// all the needed fields, unneeded fields
	f("facets", "*", "f1,f2", "*", "")

	// needed fields
	f("facets", "f1,f2", "", "*", "")
}
