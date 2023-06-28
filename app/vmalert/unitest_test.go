package main

import (
	"reflect"
	"testing"
)

func TestUnitRule(t *testing.T) {
	testCases := []struct {
		name   string
		files  []string
		failed bool
	}{
		{
			name:   "run multi files",
			files:  []string{"./unittest/testdata/test1.yaml", "./unittest/testdata/test2.yaml"},
			failed: false,
		},
		{
			name:   "failing test",
			files:  []string{"./unittest/testdata/failed-test.yaml"},
			failed: true,
		},
		{
			// This test will take about 1 minute to run now.
			// todo may need to improve the performance
			name:   "long period",
			files:  []string{"./unittest/testdata/long-period.yaml"},
			failed: false,
		},
	}
	for _, tc := range testCases {
		fail := unitRule(tc.files...)
		if fail != tc.failed {
			t.Fatalf("failed to test %s, expect %t, got %t", tc.name, tc.failed, fail)
		}
	}
}

func TestParseInputValue(t *testing.T) {
	testCases := []struct {
		input  string
		exp    []sequenceValue
		failed bool
	}{
		{
			"",
			nil,
			true,
		},
		{
			"testfailed",
			nil,
			true,
		},
		{
			"-4",
			[]sequenceValue{{value: -4}},
			false,
		},
		{
			"_",
			[]sequenceValue{{omitted: true}},
			false,
		},
		{
			"-4x1",
			[]sequenceValue{{value: -4}, {value: -4}},
			false,
		},
		{
			"_x1",
			[]sequenceValue{{omitted: true}},
			false,
		},
		{
			"1+1x4",
			[]sequenceValue{{value: 1}, {value: 2}, {value: 3}, {value: 4}, {value: 5}},
			false,
		},
		{
			"2-1x4",
			[]sequenceValue{{value: 2}, {value: 1}, {value: 0}, {value: -1}, {value: -2}},
			false,
		},
		{
			"1+1x1 _ -4 3+20x1",
			[]sequenceValue{{value: 1}, {value: 2}, {omitted: true}, {value: -4}, {value: 3}, {value: 23}},
			false,
		},
	}

	for _, tc := range testCases {
		output, err := parseInputValue(tc.input, true)
		if err != nil != tc.failed {
			t.Fatalf("failed to parse %s, expect %t, got %t", tc.input, tc.failed, err != nil)
		}
		if !reflect.DeepEqual(tc.exp, output) {
			t.Fatalf("expect %v, got %v", tc.exp, output)
		}
	}
}
