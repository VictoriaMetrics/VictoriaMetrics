package main

import (
	"testing"
)

func TestUrlValuesToStrings(t *testing.T) {
	mapQueryParams := map[string][]string{
		"param1": {"param1"},
		"param2": {"anotherparam"},
	}
	expectedRes := []string{"param1=param1", "param2=anotherparam"}
	res := urlValuesToStrings(mapQueryParams)

	if len(res) != len(expectedRes) {
		t.Errorf("Expected length %d, but got %d", len(expectedRes), len(res))
	}
	for ind, val := range expectedRes {
		if val != res[ind] {
			t.Errorf("Expected %v; but got %v", val, res[ind])
		}
	}
}
