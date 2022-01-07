package opentsdb

import (
	"testing"
)

func TestConvertRetention(t *testing.T) {
	/*
		2592000 seconds in 30 days
		3600 in one hour
		2592000 / 14400 = 180 individual query "ranges" should exist, plus one because time ranges can be weird
		First order should == "sum"
		Second order should == "avg"
		AggTime should == "1m"
	*/
	res, err := convertRetention("sum-1m-avg:1h:30d", 0, false)
	if err != nil {
		t.Fatalf("Error parsing valid retention string: %v", err)
	}
	if len(res.QueryRanges) != 181 {
		t.Fatalf("Found %v query ranges. Should have found 181", len(res.QueryRanges))
	}
	if res.FirstOrder != "sum" {
		t.Fatalf("Incorrect first order aggregation %q. Should have been 'sum'", res.FirstOrder)
	}
	if res.SecondOrder != "avg" {
		t.Fatalf("Incorrect second order aggregation %q. Should have been 'avg'", res.SecondOrder)
	}
	if res.AggTime != "1m" {
		t.Fatalf("Incorrect aggregation time length %q. Should have been '1m'", res.AggTime)
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
	/*
		Good metric metadata
	*/
	m := Metric{
		Metric: "cpu",
		Tags: map[string]string{
			"core": "0",
		},
		Values: []float64{
			0,
		},
		Timestamps: []int64{
			0,
		},
	}
	res, err := modifyData(m, false)
	if err != nil {
		t.Fatalf("Valid metric %v failed to parse: %v", m, err)
	}
	if res.Metric != "cpu" {
		t.Fatalf("Valid metric name %q was converted: %q", m.Metric, res.Metric)
	}
	found := false
	for k := range res.Tags {
		if k == "core" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Valid metric tag name 'core' missing: %v", res.Tags)
	}

	/*
		Bad first character in metric name
		metric names cannot start with _, so this
		metric should fail entirely
	*/
	m = Metric{
		Metric: "_cpu",
		Tags: map[string]string{
			"core": "0",
		},
		Values: []float64{
			0,
		},
		Timestamps: []int64{
			0,
		},
	}
	res, err = modifyData(m, false)
	if err == nil {
		t.Fatalf("Invalid metric %v parsed?", m)
	}

	/*
		Bad character in metric name
		metric names cannot have `.`, so this
		should be converted to `_`
	*/
	m = Metric{
		Metric: "cpu.name",
		Tags: map[string]string{
			"core": "0",
		},
		Values: []float64{
			0,
		},
		Timestamps: []int64{
			0,
		},
	}
	res, err = modifyData(m, false)
	if err != nil {
		t.Fatalf("Valid metric failed to parse? %v", err)
	}
	if res.Metric != "cpu_name" {
		t.Fatalf("Metric name not correctly converted from 'cpu.name' to 'cpu_name': %q", res.Metric)
	}

	/*
		bad tag prefix (__)
		Prometheus considers tags beginning with __
		to be internal use only. They should not show up in incoming data.
		this tag should be dropped from the result
	*/
	m = Metric{
		Metric: "cpu",
		Tags: map[string]string{
			"__core": "0",
		},
		Values: []float64{
			0,
		},
		Timestamps: []int64{
			0,
		},
	}
	res, err = modifyData(m, false)
	if err != nil {
		t.Fatalf("Valid metric failed to parse? %v", err)
	}
	found = false
	for k := range res.Tags {
		if k == "__core" {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("Bad tag key prefix (__) found")
	}

	/*
		bad tag key
		tag keys cannot contain `.`, this should be
		replaced with `_`
	*/
	m = Metric{
		Metric: "cpu",
		Tags: map[string]string{
			"core.name": "0",
		},
		Values: []float64{
			0,
		},
		Timestamps: []int64{
			0,
		},
	}
	res, err = modifyData(m, false)
	if err != nil {
		t.Fatalf("Valid metric failed to parse? %v", err)
	}
	found = false
	for k := range res.Tags {
		if k == "core.name" {
			found = true
			break
		}
	}
	if found {
		t.Fatalf("Bad tag key 'core.name' not converted")
	}

	/*
		test normalize
		All characters should be returned lowercase
	*/
	m = Metric{
		Metric: "CPU",
		Tags: map[string]string{
			"core": "0",
		},
		Values: []float64{
			0,
		},
		Timestamps: []int64{
			0,
		},
	}
	res, err = modifyData(m, true)
	if err != nil {
		t.Fatalf("Valid metric failed to parse? %v", err)
	}
	if res.Metric != "cpu" {
		t.Fatalf("Normalization of metric name didn't happen!")
	}
}
