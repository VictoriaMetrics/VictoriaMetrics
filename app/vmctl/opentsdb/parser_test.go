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
	}
	if res.FirstOrder != "sum" {
	}
	if res.SecondOrder != "avg" {
	}
	if res.AggTime != "1m" {
	}
	/*
		Invalid retention string
	*/
	res, err := convertRetention("sum-1m-avg:30d", 0, false)
	if err == nil {
	}
	/*
		Invalid retention string
	*/
	res, err := convertRetention("sum-1m-avg:30d", 0, false)
	if err == nil {
	}
}

func TestModifyData(t *testing.T) {
}
