package opentsdb

import (
	"testing"
)

func TestConvertRetention(t *testing.T) {
	/*
		2592000 seconds in 30 days
		3600 in one hour
		2592000 / 3600 = 720 individual query "ranges" should exist
		First order should == "sum"
		Second order should == "avg"
		AggTime should == "1m"
	*/
	res, err := convertRetention("sum-1m-avg:1h:30d", 0, false)
	if len(res.QueryRanges) != 720 {
		t.Fatalf("Found %v query ranges. Should have found 720", len(res.QueryRanges))
	}
	if res.FirstOrder != "sum" {
		t.Fatalf("Incorrect first order aggregation %v. Should have been sum", res.FirstOrder)
	}
	if res.SecondOrder != "avg" {
		t.Fatalf("Incorrect second order aggregation %v. Should have been avg", res.SecondOrder)
	}
	if res.AggTime != "1m" {
		t.Fatalf("Incorrect aggregation time length %v. Should have been 1m", res.AggTime)
	}
	/*
		Invalid retention string
	*/
	res, err = convertRetention("sum-1m-avg:30d", 0, false)
	if err == nil {
		t.Fatalf("Bad retention string (sum-1m-avg:30d) didn't fail: %v", res)
	}
	/*
		Invalid aggregation string
	*/
	res, err = convertRetention("sum-1m:1h:30d", 0, false)
	if err == nil {
		t.Fatalf("Bad aggregation string (sum-1m:1h:30d) didn't fail: %v", res)
	}
}

func TestModifyData(t *testing.T) {
}
