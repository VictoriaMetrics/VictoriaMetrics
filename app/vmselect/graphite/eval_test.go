package graphite

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/graphiteql"
)

func TestExecExprSuccess(t *testing.T) {
	ec := &evalConfig{
		startTime:   120e3,
		endTime:     210e3,
		storageStep: 30e3,
		currentTime: time.Unix(150e3, 0),
	}
	f := func(query string, expectedSeries []*series) {
		t.Helper()
		ecCopy := *ec
		nextSeries, err := execExpr(&ecCopy, query)
		if err != nil {
			t.Fatalf("unexpected error in execExpr(%q): %s", query, err)
		}
		ss, err := fetchAllSeries(nextSeries)
		if err != nil {
			t.Fatalf("cannot fetch all series: %s", err)
		}
		expr, err := graphiteql.Parse(query)
		if err != nil {
			t.Fatalf("cannot parse query %q: %s", query, err)
		}
		if err := compareSeries(ss, expectedSeries, expr); err != nil {
			t.Fatalf("series mismatch for query %q: %s\ngot series\n%s\nexpected series\n%s", query, err, printSeriess(ss), printSeriess(expectedSeries))
		}
		// make sure ec isn't changed during query exection.
		if !reflect.DeepEqual(ec, &ecCopy) {
			t.Fatalf("unexpected ec\ngot\n%v\nwant\n%v", &ecCopy, ec)
		}
	}

	f("absolute(constantLine(-1.23))", []*series{
		{
			Timestamps: []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:     []float64{1.23, 1.23, 1.23},
			Name:       "absolute(-1.23)",
			Tags:       map[string]string{"name": "-1.23", "absolute": "1"},
		},
	})
	f("add(constantLine(1.23), 4.57)", []*series{
		{
			Timestamps:     []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:         []float64{5.8, 5.8, 5.8},
			Name:           "add(1.23,4.57)",
			Tags:           map[string]string{"name": "1.23", "add": "4.57"},
			pathExpression: "add(1.23,4.57)",
		},
	})
	f("add(constantLine(-123), constant=-457)", []*series{
		{
			Timestamps:     []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:         []float64{-580, -580, -580},
			Name:           "add(-123,-457)",
			Tags:           map[string]string{"name": "-123", "add": "-457"},
			pathExpression: "add(-123,-457)",
		},
	})
	f(`aggregate(
		group(
			constantLine(1)|alias("foo"),
			constantLine(2)|alias("bar;aa=bb")
		),
		"sum"
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{3, 3},
			Name:       "sumSeries(constantLine(1),constantLine(2))",
			Tags:       map[string]string{"name": "sumSeries(constantLine(1),constantLine(2))", "aggregatedBy": "sum"},
		},
	})
	f(`aggregate(
		group(
			constantLine(1)|alias("foo"),
			time("bar", 10),
		),
		"count",
		xFilesFactor = 1,
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{2, 2},
			Name:       "countSeries(bar,constantLine(1))",
			Tags:       map[string]string{"name": "countSeries(bar,constantLine(1))", "aggregatedBy": "count"},
		},
	})
	f(`aggregate(
		group(
			constantLine(1)|alias("foo"),
			time("bar", 10)
		),
		"avg_zero"
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{70.5, 93},
			Name:       "avg_zeroSeries(bar,constantLine(1))",
			Tags:       map[string]string{"name": "avg_zeroSeries(bar,constantLine(1))", "aggregatedBy": "avg_zero"},
		},
	})
	f(`aggregate(
		group(
			constantLine(1)|alias("foo"),
			time("bar", 10)
		),
		"min"
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{1, 1},
			Name:       "minSeries(bar,constantLine(1))",
			Tags:       map[string]string{"name": "minSeries(bar,constantLine(1))", "aggregatedBy": "min"},
		},
	})
	f(`aggregate(
		group(
			constantLine(1)|alias("foo"),
			time("bar", 10),
		),
		"diff",
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{-139, -184},
			Name:       "diffSeries(constantLine(1),bar)",
			Tags:       map[string]string{"name": "diffSeries(constantLine(1),bar)", "aggregatedBy": "diff"},
		},
	})
	f(`aggregate(
		group(
			constantLine(1)|alias("foo"),
			time("bar", 10),
		),
		"range",
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{139, 184},
			Name:       "rangeSeries(bar,constantLine(1))",
			Tags:       map[string]string{"name": "rangeSeries(bar,constantLine(1))", "aggregatedBy": "range"},
		},
	})
	f(`aggregate(
		group(
			constantLine(2)|alias("foo"),
			time("bar", 10),
		),
		"multiply",
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{280, 370},
			Name:       "multiplySeries(bar,constantLine(2))",
			Tags:       map[string]string{"name": "multiplySeries(bar,constantLine(2))", "aggregatedBy": "multiply"},
		},
	})
	f(`aggregate(
		group(
			constantLine(2)|alias("foo"),
			time("bar", 10),
		),
		"first",
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{2, 2},
			Name:       "firstSeries(constantLine(2),bar)",
			Tags:       map[string]string{"name": "firstSeries(constantLine(2),bar)", "aggregatedBy": "first"},
		},
	})
	f(`aggregate(
		group(
			constantLine(2)|alias("foo"),
			time("bar", 10),
		),
		"last",
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{140, 185},
			Name:       "lastSeries(constantLine(2),bar)",
			Tags:       map[string]string{"name": "lastSeries(constantLine(2),bar)", "aggregatedBy": "last"},
		},
	})
	f("aggregate(group(),'avg')", []*series{})
	f(`aggregateLine(
		group(
			time("foo", 10),
			time("bar", 25),
		)
	)`, []*series{
		{
			Timestamps: []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:     []float64{165, 165, 165},
			Name:       "aggregateLine(foo,165)",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps: []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:     []float64{157.5, 157.5, 157.5},
			Name:       "aggregateLine(bar,157.5)",
			Tags:       map[string]string{"name": "bar"},
		},
	})
	f(`aggregateLine(constantLine(1),"count")`, []*series{
		{
			Timestamps: []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:     []float64{3, 3, 3},
			Name:       "aggregateLine(1,3)",
			Tags:       map[string]string{"name": "1"},
		},
	})
	f(`aggregateLine(time('foo',10),"median")`, []*series{
		{
			Timestamps: []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:     []float64{170, 170, 170},
			Name:       "aggregateLine(foo,170)",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`aggregateLine(time('foo',10),"max")`, []*series{
		{
			Timestamps: []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:     []float64{210, 210, 210},
			Name:       "aggregateLine(foo,210)",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`aggregateLine(time('foo',10),"diff")`, []*series{
		{
			Timestamps: []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:     []float64{-1410, -1410, -1410},
			Name:       "aggregateLine(foo,-1410)",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`aggregateLine(time('foo',10),"stddev")`, []*series{
		{
			Timestamps: []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:     []float64{28.722813232690143, 28.722813232690143, 28.722813232690143},
			Name:       "aggregateLine(foo,28.722813232690143)",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`aggregateLine(time('foo',10),"range")`, []*series{
		{
			Timestamps: []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:     []float64{90, 90, 90},
			Name:       "aggregateLine(foo,90)",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`aggregateLine(time('foo',10),"multiply")`, []*series{
		{
			Timestamps: []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:     []float64{1.2799358208e+22, 1.2799358208e+22, 1.2799358208e+22},
			Name:       "aggregateLine(foo,1.2799358208e+22)",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`aggregateLine(time("foo",20),func="min",keepStep=True)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{120, 120, 120, 120, 120},
			Name:       "aggregateLine(foo,120)",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`aggregateWithWildcards(
		group(
			time("foo.bar", 30),
			time("foo.baz", 60)
		),
		func='max'
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, nan, 180},
			Name:       "foo.baz",
			Tags:       map[string]string{"name": "foo.baz", "aggregatedBy": "max"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "foo.bar",
			Tags:       map[string]string{"name": "foo.bar", "aggregatedBy": "max"},
		},
	})
	f(`aggregateWithWildcards(
		group(
			time("foo.bar", 30),
			time("foo.baz", 60)
		),
		func='median',
		1
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "foo",
			Tags:           map[string]string{"name": "medianSeries(foo.bar,foo.baz)", "aggregatedBy": "median"},
			pathExpression: "medianSeries(foo.bar,foo.baz)",
		},
	})
	f(`aggregateWithWildcards(
		group(
			time("foo.bar", 30),
			time("foo.baz", 60)
		),
		func='stddev',
		1, 0, 2
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{0, 0, 0},
			Name:           "",
			Tags:           map[string]string{"aggregatedBy": "stddev", "name": "stddevSeries(foo.bar,foo.baz)"},
			pathExpression: "stddevSeries(foo.bar,foo.baz)",
		},
	})

	f("alias(constantLine(123), 'foo.bar;baz=aaa')", []*series{
		{
			Timestamps:     []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:         []float64{123, 123, 123},
			Name:           "foo.bar;baz=aaa",
			Tags:           map[string]string{"name": "123"},
			pathExpression: "constantLine(123)",
		},
	})
	f(`aliasByMetric(
		group(
			time("foo.bar.baz;x=y"),
			time("aaa.bb", 30),
			summarize(group(time('a'),time('c.d.b')),'30s'),
		)
	)`, []*series{
		{
			Timestamps:     []int64{ec.startTime, ec.startTime + 60*1000},
			Values:         []float64{120, 180},
			Name:           "baz;x=y",
			Tags:           map[string]string{"name": "foo.bar.baz", "x": "y"},
			pathExpression: "foo.bar.baz;x=y",
		},
		{
			Timestamps:     []int64{ec.startTime, ec.startTime + 30*1000, ec.startTime + 60*1000, ec.startTime + 90*1000},
			Values:         []float64{120, 150, 180, 210},
			Name:           "bb",
			Tags:           map[string]string{"name": "aaa.bb"},
			pathExpression: "aaa.bb",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000, 210000},
			Values:         []float64{120, nan, 180, nan},
			Name:           "a",
			Tags:           map[string]string{"name": "a", "summarize": "30s", "summarizeFunction": "sum"},
			pathExpression: "summarize(a,'30s','sum')",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000, 210000},
			Values:         []float64{120, nan, 180, nan},
			Name:           "b",
			Tags:           map[string]string{"name": "c.d.b", "summarize": "30s", "summarizeFunction": "sum"},
			pathExpression: "summarize(c.d.b,'30s','sum')",
		},
	})
	f(`aliasByMetric(
                 summarize(
                  exclude(
                    groupByNode(
                      time("svc.default.first.prod.srv.1.http.returned-codes.500"),
                      8,
                      'sum'), 
                 '200'),
                '5min',
                'sum',
                false))`, []*series{
		{
			Timestamps: []int64{0},
			Values:     []float64{600},
			Name:       "500",
			Tags: map[string]string{
				"aggregatedBy":      "sum",
				"name":              "svc.default.first.prod.srv.1.http.returned-codes.500",
				"summarize":         "5min",
				"summarizeFunction": "sum",
			},
			pathExpression: "summarize(500,'5min','sum')",
		},
	})

	f(`aliasByNode(time("foo.bar.baz"))`, []*series{
		{
			Timestamps:     []int64{ec.startTime, ec.startTime + 60*1000},
			Values:         []float64{120, 180},
			Name:           "",
			Tags:           map[string]string{"name": "foo.bar.baz"},
			pathExpression: "foo.bar.baz",
		},
	})
	f(`aliasByTags(
		group(
			time("foo.bar.baz;aa=bb", 20),
			time("foo.xx", 50)
		),
		1, "aa"
	)`, []*series{
		{
			Timestamps:     []int64{120000, 140000, 160000, 180000, 200000},
			Values:         []float64{120, 140, 160, 180, 200},
			Name:           "bar.bb",
			Tags:           map[string]string{"name": "foo.bar.baz", "aa": "bb"},
			pathExpression: "foo.bar.baz;aa=bb",
		},
		{
			Timestamps:     []int64{120000, 170000},
			Values:         []float64{120, 170},
			Name:           "xx",
			Tags:           map[string]string{"name": "foo.xx"},
			pathExpression: "foo.xx",
		},
	})
	f(`aliasQuery(
		group(
			time("foo.1.2", 20),
			time("foo.3.4", 50),
		),
		"foo\.([^.]+\.[^.]+)",
		"constantLine(\1)|alias('aaa.\1')",
		"foo %d bar %g"
	)`, []*series{
		{
			Timestamps:     []int64{120000, 140000, 160000, 180000, 200000},
			Values:         []float64{120, 140, 160, 180, 200},
			Name:           "foo 1 bar 1.2",
			Tags:           map[string]string{"name": "foo.1.2"},
			pathExpression: "foo.1.2",
		},
		{
			Timestamps:     []int64{120000, 170000},
			Values:         []float64{120, 170},
			Name:           "foo 3 bar 3.4",
			Tags:           map[string]string{"name": "foo.3.4"},
			pathExpression: "foo.3.4",
		},
	})
	f(`aliasSub(
		group(
			time("foo.1.2", 20),
			time("foo.3.4", 50),
		),
		"foo\.([^.]+)\.([^.]+)",
		"bar\2\1.x\2"
	)`, []*series{
		{
			Timestamps:     []int64{120000, 140000, 160000, 180000, 200000},
			Values:         []float64{120, 140, 160, 180, 200},
			Name:           "bar21.x2",
			Tags:           map[string]string{"name": "foo.1.2"},
			pathExpression: "foo.1.2",
		},
		{
			Timestamps:     []int64{120000, 170000},
			Values:         []float64{120, 170},
			Name:           "bar43.x4",
			Tags:           map[string]string{"name": "foo.3.4"},
			pathExpression: "foo.3.4",
		},
	})
	f(`alpha(time("foo",50),0.5)`, []*series{
		{
			Timestamps: []int64{120000, 170000},
			Values:     []float64{120, 170},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`applyByNode(
		time("foo.bar.baz",25),
		1,
		"time('%.abc;de=fg',50)"
	)`, []*series{
		{
			Timestamps:     []int64{120000, 170000},
			Values:         []float64{120, 170},
			Name:           "foo.bar.abc;de=fg",
			Tags:           map[string]string{"name": "foo.bar.abc", "de": "fg"},
			pathExpression: "foo.bar",
		},
	})
	f(`applyByNode(
		time("foo.bar.baz",25),
		1,
		"time('%.abc;de=fg',50)",
		"a.%.end"
	)`, []*series{
		{
			Timestamps:     []int64{120000, 170000},
			Values:         []float64{120, 170},
			Name:           "a.foo.bar.end",
			Tags:           map[string]string{"name": "foo.bar.abc", "de": "fg"},
			pathExpression: "foo.bar",
		},
	})
	f(`areaBetween(
		group(
			time("a"),
			time("b"),
		)
	)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "areaBetween(a)",
			Tags:       map[string]string{"name": "a", "areaBetween": "1"},
		},
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "areaBetween(b)",
			Tags:       map[string]string{"name": "b", "areaBetween": "1"},
		},
	})
	f(`asPercent(
		group(
			time("foo", 30),
			time("bar", 30),
		)
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{50, 50, 50},
			Name:       "asPercent(foo,sumSeries(bar,foo))",
			Tags:       map[string]string{"name": "asPercent(foo,sumSeries(bar,foo))"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{50, 50, 50},
			Name:       "asPercent(bar,sumSeries(bar,foo))",
			Tags:       map[string]string{"name": "asPercent(bar,sumSeries(bar,foo))"},
		},
	})
	f(`asPercent(
		group(
			time("foo", 17),
			time("bar", 23),
		),
		150
	)`, []*series{
		{
			Timestamps: []int64{120000, 137000, 154000, 171000, 188000, 205000},
			Values:     []float64{80, 91.33333333333333, 102.66666666666666, 113.99999999999999, 125.33333333333334, 136.66666666666666},
			Name:       "asPercent(foo,150)",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps: []int64{120000, 143000, 166000, 189000},
			Values:     []float64{80, 95.33333333333334, 110.66666666666667, 126},
			Name:       "asPercent(bar,150)",
			Tags:       map[string]string{"name": "bar"},
		},
	})
	f(`asPercent(
		group(
			time("foo.x", 30),
			time("bar.x", 30),
			time("bar.y", 30),
		),
		group(),
	)`, []*series{})
	f(`asPercent(
		group(
			time("foo.x", 30),
			time("bar.x", 30),
			time("bar.y", 30),
		),
		None,
		0
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{100, 100, 100},
			Name:       "asPercent(foo.x,foo.x)",
			Tags:       map[string]string{"name": "asPercent(foo.x,foo.x)"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{50, 50, 50},
			Name:       "asPercent(bar.x,sumSeries(bar.x,bar.y))",
			Tags:       map[string]string{"name": "asPercent(bar.x,sumSeries(bar.x,bar.y))"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{50, 50, 50},
			Name:       "asPercent(bar.y,sumSeries(bar.x,bar.y))",
			Tags:       map[string]string{"name": "asPercent(bar.y,sumSeries(bar.x,bar.y))"},
		},
	})
	f(`asPercent(
		group(
			time("foo;a=b", 30),
			time("bar", 30)
		),
		constantLine(100)|alias("baz;x=y")
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{135, 180},
			Name:       "asPercent(bar,baz;x=y)",
			Tags:       map[string]string{"name": "asPercent(bar,baz;x=y)"},
		},
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{135, 180},
			Name:       "asPercent(foo;a=b,baz;x=y)",
			Tags:       map[string]string{"name": "asPercent(foo;a=b,baz;x=y)", "a": "b"},
		},
	})
	f(`asPercent(
		group(
			time("foo", 30),
			time("bar", 30),
		),
		group(
			time("x", 30),
			time("y", 30),
		)
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{100, 100, 100},
			Name:       "asPercent(bar,y)",
			Tags:       map[string]string{"name": "asPercent(bar,y)"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{100, 100, 100},
			Name:       "asPercent(foo,x)",
			Tags:       map[string]string{"name": "asPercent(foo,x)"},
		},
	})
	f(`asPercent(
		group(
			time("foo.x;c=d", 30),
			time("bar.b;a=b", 30),
			time("bar.a", 30)
		),
		group(
			time("bar.sss", 30),
			time("abc;e=g", 30)
		),
		0
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{100, 100, 100},
			Name:       "asPercent(bar.b;a=b,bar.sss)",
			Tags:       map[string]string{"name": "asPercent(bar.b;a=b,bar.sss)", "a": "b"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{100, 100, 100},
			Name:       "asPercent(bar.a,bar.sss)",
			Tags:       map[string]string{"name": "asPercent(bar.a,bar.sss)"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{nan, nan, nan},
			Name:       `asPercent(MISSING,abc;e=g)`,
			Tags:       map[string]string{"name": `asPercent(MISSING,abc;e=g)`},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{nan, nan, nan},
			Name:       "asPercent(foo.x;c=d,MISSING)",
			Tags:       map[string]string{"name": "asPercent(foo.x;c=d,MISSING)", "c": "d"},
		},
	})
	f(`averageAbove(
		group(
			time('foo'),
			constantLine(10)|alias('bar'),
			time('baz', 20)|add(100),
		),
		160
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{220, 240, 260, 280, 300},
			Name:       "add(baz,100)",
			Tags:       map[string]string{"name": "baz", "add": "100"},
		},
	})
	f(`averageBelow(
		group(
			time('foo'),
			constantLine(10)|alias('bar'),
			time('baz', 20)|add(100),
		),
		160
	)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps:     []int64{120000, 165000, 210000},
			Values:         []float64{10, 10, 10},
			Name:           "bar",
			Tags:           map[string]string{"name": "10"},
			pathExpression: "constantLine(10)",
		},
	})
	f(`averageOutsidePercentile(
		group(
			add(time('a'),-10),
			time('b'),
			add(time('c'),10),
			add(time('d'),20),
		),
		75
	)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{110, 170},
			Name:       "add(a,-10)",
			Tags:       map[string]string{"name": "a", "add": "-10"},
		},
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{140, 200},
			Name:       "add(d,20)",
			Tags:       map[string]string{"name": "d", "add": "20"},
		},
	})
	f(`averageSeries(
		time('foo',30),
		time('bar',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "averageSeries(bar,foo)",
			Tags:       map[string]string{"name": "averageSeries(bar,foo)", "aggregatedBy": "average"},
		},
	})
	f(`averageSeriesWithWildcards(
		group(
			time('foo.bar',30),
			time('foo.baz',30),
			time('xxx.yy',30),
		),
		1
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "xxx",
			Tags:           map[string]string{"aggregatedBy": "average", "name": "xxx.yy"},
			pathExpression: "xxx.yy",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "foo",
			Tags:           map[string]string{"aggregatedBy": "average", "name": "averageSeries(foo.bar,foo.baz)"},
			pathExpression: "averageSeries(foo.bar,foo.baz)",
		},
	})
	f(`averageSeriesWithWildcards(
		group(
			time('foo.bar',30),
			time('foo.baz',30),
			time('xxx.yy',30),
		)
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "xxx.yy",
			Tags:       map[string]string{"aggregatedBy": "average", "name": "xxx.yy"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "foo.bar",
			Tags:       map[string]string{"aggregatedBy": "average", "name": "foo.bar"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "foo.baz",
			Tags:       map[string]string{"aggregatedBy": "average", "name": "foo.baz"},
		},
	})
	f(`avg(
		group(
			time('foo',30),
			time('xxx',30),
		),
		time('bar',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "averageSeries(bar,foo,xxx)",
			Tags:       map[string]string{"name": "averageSeries(bar,foo,xxx)", "aggregatedBy": "average"},
		},
	})
	f(`changed(
		group(
			constantLine(123)|alias('foo'),
			time('bar')
		)
	)`, []*series{
		{
			Timestamps:     []int64{120000, 165000, 210000},
			Values:         []float64{0, 0, 0},
			Name:           "changed(foo)",
			Tags:           map[string]string{"name": "123"},
			pathExpression: "changed(foo)",
		},
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{0, 1},
			Name:           "changed(bar)",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "changed(bar)",
		},
	})
	f(`color(time("foo",50),'green')`, []*series{
		{
			Timestamps: []int64{120000, 170000},
			Values:     []float64{120, 170},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`averageSeries(
		consolidateBy(
			group(
				time('foo',30),
				time('bar',30)
			),
			'first'
		)
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       `averageSeries(consolidateBy(bar,'first'),consolidateBy(foo,'first'))`,
			Tags: map[string]string{
				"name":          "averageSeries(consolidateBy(bar,'first'),consolidateBy(foo,'first'))",
				"aggregatedBy":  "average",
				"consolidateBy": "first",
			},
		},
	})
	f(`constantLine(123) | alias("foo.bar;baz=aaa")`, []*series{
		{
			Timestamps:     []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:         []float64{123, 123, 123},
			Name:           "foo.bar;baz=aaa",
			Tags:           map[string]string{"name": "123"},
			pathExpression: "constantLine(123)",
		},
	})
	f("constantLine(123.456)", []*series{
		{
			Timestamps:     []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:         []float64{123.456, 123.456, 123.456},
			Name:           "123.456",
			Tags:           map[string]string{"name": "123.456"},
			pathExpression: "constantLine(123.456)",
		},
	})
	f("constantLine(value=-123)", []*series{
		{
			Timestamps:     []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:         []float64{-123, -123, -123},
			Name:           "-123",
			Tags:           map[string]string{"name": "-123"},
			pathExpression: "constantLine(value=-123)",
		},
	})
	f(`countSeries(
		time('foo',30),
		time('bar',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{2, 2, 2},
			Name:       "countSeries(bar,foo)",
			Tags:       map[string]string{"name": "countSeries(bar,foo)", "aggregatedBy": "count"},
		},
	})
	f(`averageSeries(
		cumulative(
			time('foo', 30)
		)
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       `averageSeries(consolidateBy(foo,'sum'))`,
			Tags:       map[string]string{"name": "foo", "aggregatedBy": "average", "consolidateBy": "sum"},
		},
	})
	f(`currentAbove(
		group(
			time('foo'),
			constantLine(10)|alias('bar'),
			time('baz', 20)|add(100),
		),
		200
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{220, 240, 260, 280, 300},
			Name:       "add(baz,100)",
			Tags:       map[string]string{"name": "baz", "add": "100"},
		},
	})
	f(`currentBelow(
		group(
			time('foo'),
			constantLine(10)|alias('bar'),
			time('baz', 20)|add(100),
		),
		200
	)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps:     []int64{120000, 165000, 210000},
			Values:         []float64{10, 10, 10},
			Name:           "bar",
			Tags:           map[string]string{"name": "10"},
			pathExpression: "constantLine(10)",
		},
	})
	f(`dashed(time('foo'))`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, 180},
			Name:           "dashed(foo,5)",
			Tags:           map[string]string{"name": "foo", "dashed": "5"},
			pathExpression: "foo",
		},
	})
	f(`delay(time('foo',20),1)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{nan, 120, 140, 160, 180},
			Name:       "delay(foo,1)",
			Tags:       map[string]string{"name": "foo", "delay": "1"},
		},
	})
	f(`delay(time('foo',20),-1)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{140, 160, 180, 200, nan},
			Name:       "delay(foo,-1)",
			Tags:       map[string]string{"name": "foo", "delay": "-1"},
		},
	})
	f(`delay(time('foo',20),0)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{120, 140, 160, 180, 200},
			Name:       "delay(foo,0)",
			Tags:       map[string]string{"name": "foo", "delay": "0"},
		},
	})
	f(`delay(time('foo',20),100)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{nan, nan, nan, nan, nan},
			Name:       "delay(foo,100)",
			Tags:       map[string]string{"name": "foo", "delay": "100"},
		},
	})
	f(`delay(time('foo',20),-100)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{nan, nan, nan, nan, nan},
			Name:       "delay(foo,-100)",
			Tags:       map[string]string{"name": "foo", "delay": "-100"},
		},
	})
	f(`derivative(time('foo',25))`, []*series{
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{nan, 25, 25, 25},
			Name:       "derivative(foo)",
			Tags:       map[string]string{"name": "foo", "derivative": "1"},
		},
	})
	f(`diffSeries(
		time('foo',30),
		time('bar',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{0, 0, 0},
			Name:       "diffSeries(foo,bar)",
			Tags:       map[string]string{"name": "diffSeries(foo,bar)", "aggregatedBy": "diff"},
		},
	})
	f(`divideSeries(
		group(
			time('foo',30),
			time('bar',30)
		),
		add(time('xx',30),100)
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{0.5454545454545454, 0.6, 0.6428571428571429},
			Name:       "divideSeries(foo,add(xx,100))",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{0.5454545454545454, 0.6, 0.6428571428571429},
			Name:       "divideSeries(bar,add(xx,100))",
			Tags:       map[string]string{"name": "bar"},
		},
	})
	f(`divideSeries(
		group(
			time('foo',20),
			time('bar',30)
		),
		group()
	)`, []*series{
		{
			Timestamps:     []int64{120000, 140000, 160000, 180000, 200000},
			Values:         []float64{nan, nan, nan, nan, nan},
			Name:           "divideSeries(foo,MISSING)",
			Tags:           map[string]string{"name": "foo"},
			pathExpression: "divideSeries(foo,MISSING)",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000, 210000},
			Values:         []float64{nan, nan, nan, nan},
			Name:           "divideSeries(bar,MISSING)",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "divideSeries(bar,MISSING)",
		},
	})
	f(`divideSeriesLists(
		group(
			time('foo',30),
			time('bar',30)
		),
		group(
			time('xx',30),
			time('y',30),
		)
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{1, 1, 1},
			Name:       "divideSeries(foo,xx)",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{1, 1, 1},
			Name:       "divideSeries(bar,y)",
			Tags:       map[string]string{"name": "bar"},
		},
	})
	f(`drawAsInfinite(time('a'))`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, 180},
			Name:           "drawAsInfinite(a)",
			Tags:           map[string]string{"name": "a", "drawAsInfinite": "1"},
			pathExpression: "a",
		},
	})
	f(`events()`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{nan, nan, nan},
			Name:       "events()",
			Tags:       map[string]string{"name": "events()"},
		},
	})
	f(`events("foo","bar")`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{nan, nan, nan},
			Name:       "events('foo','bar')",
			Tags:       map[string]string{"name": "events('foo','bar')"},
		},
	})
	f(`exclude(
		group(
			time("foo.bar.baz"),
			time("x"),
		),
		"bar"
	)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "x",
			Tags:       map[string]string{"name": "x"},
		},
	})
	f(`exp(scale(time('a',25),1e-2))`, []*series{
		{
			Timestamps:     []int64{120000, 145000, 170000, 195000},
			Values:         []float64{3.3201169227365472, 4.263114515168817, 5.4739473917272, 7.028687580589293},
			Name:           "exp(scale(a,0.01))",
			Tags:           map[string]string{"name": "a", "exp": "e"},
			pathExpression: "exp(scale(a,0.01))",
		},
	})
	f(`exponentialMovingAverage(time('a',20),'1min')`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{81.31147540983606, 83.23568933082504, 85.75255197571603, 88.8426322388073, 92.48713609983001},
			Name:       "exponentialMovingAverage(a,'1min')",
			Tags:       map[string]string{"name": "a", "exponentialMovingAverage": "'1min'"},
		},
	})
	f(`exponentialMovingAverage(time('a',20),'10s')`, []*series{
		{
			Timestamps: []int64{130000, 150000, 170000, 190000, 210000},
			Values:     []float64{113.63636363636364, 120.24793388429751, 129.2937640871525, 140.33126152585203, 152.998304884788},
			Name:       "exponentialMovingAverage(a,'10s')",
			Tags:       map[string]string{"name": "a", "exponentialMovingAverage": "'10s'"},
		},
	})
	f(`exponentialMovingAverage(time('a',20),5)`, []*series{
		{
			Timestamps: []int64{130000, 150000, 170000, 190000, 210000},
			Values:     []float64{70, 96.66666666666667, 121.11111111111111, 144.07407407407408, 166.0493827160494},
			Name:       "exponentialMovingAverage(a,5)",
			Tags:       map[string]string{"name": "a", "exponentialMovingAverage": "5"},
		},
	})
	f(`fallbackSeries(time('a'),constantLine(10))`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "a",
			Tags:       map[string]string{"name": "a"},
		},
	})
	f(`fallbackSeries(group(),constantLine(10))`, []*series{
		{
			Timestamps:     []int64{120000, 165000, 210000},
			Values:         []float64{10, 10, 10},
			Name:           "10",
			Tags:           map[string]string{"name": "10"},
			pathExpression: "constantLine(10)",
		},
	})
	f(`filterSeries(
		group(
			time('a',20),
			add(time('b',20),200),
		),
		'last','>=',300
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{320, 340, 360, 380, 400},
			Name:       "add(b,200)",
			Tags:       map[string]string{"name": "b", "add": "200"},
		},
	})
	f(`filterSeries(
		group(
			time('a',20),
			add(time('b',20),200),
		),
		'first','<=',120
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{120, 140, 160, 180, 200},
			Name:       "a",
			Tags:       map[string]string{"name": "a"},
		},
	})
	f(`filterSeries(
		group(
			time('a',20),
			add(time('b',20),200),
		),
		'first','=',120
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{120, 140, 160, 180, 200},
			Name:       "a",
			Tags:       map[string]string{"name": "a"},
		},
	})
	f(`filterSeries(
		group(
			time('a',20),
			add(time('b',20),200),
		),
		'first','!=',120
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{320, 340, 360, 380, 400},
			Name:       "add(b,200)",
			Tags:       map[string]string{"name": "b", "add": "200"},
		},
	})
	f(`grep(
		group(
			time("foo.bar.baz"),
			time("x"),
		),
		"bar"
	)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "foo.bar.baz",
			Tags:       map[string]string{"name": "foo.bar.baz"},
		},
	})
	f("group()", []*series{})
	f("group(constantLine(1)|alias('foo'), constantLine(2) | alias('bar'))", []*series{
		{
			Timestamps:     []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:         []float64{1, 1, 1},
			Name:           "foo",
			Tags:           map[string]string{"name": "1"},
			pathExpression: "constantLine(1)",
		},
		{
			Timestamps:     []int64{ec.startTime, (ec.startTime + ec.endTime) / 2, ec.endTime},
			Values:         []float64{2, 2, 2},
			Name:           "bar",
			Tags:           map[string]string{"name": "2"},
			pathExpression: "constantLine(2)",
		},
	})
	f(`groupByNode(
		group(
			time("foo.bar", 30),
			time("foo.baz", 30)
		),
		0
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "foo",
			Tags:           map[string]string{"aggregatedBy": "average", "name": "averageSeries(foo.bar,foo.baz)"},
			pathExpression: "averageSeries(foo.bar,foo.baz)",
		},
	})
	f(`groupByNode(
		group(
			time("foo.bar", 30),
			time("foo.baz", 30)
		),
		0,
		'last'
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "foo",
			Tags:           map[string]string{"aggregatedBy": "last", "name": "lastSeries(foo.bar,foo.baz)"},
			pathExpression: "lastSeries(foo.bar,foo.baz)",
		},
	})
	f(`groupByNodes(
		group(
			time("foo.bar", 30),
			time("foo.baz", 30)
		),
		callback='first',
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "",
			Tags:           map[string]string{"aggregatedBy": "first", "name": "firstSeries(foo.bar,foo.baz)"},
			pathExpression: "firstSeries(foo.bar,foo.baz)",
		},
	})
	f(`groupByNodes(
		group(
			time("foo.bar", 30),
			time("foo.baz", 30)
		),
		'median',
		0
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "foo",
			Tags:           map[string]string{"aggregatedBy": "median", "name": "medianSeries(foo.bar,foo.baz)"},
			pathExpression: "medianSeries(foo.bar,foo.baz)",
		},
	})
	f(`groupByTags(
		group(
			time("foo;bar=baz", 30),
			time("x;bar=baz;aa=bb", 30)
		),
		'median',
		'bar'
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "median;bar=baz",
			Tags:           map[string]string{"aggregatedBy": "median", "bar": "baz", "name": `medianSeries(foo;bar=baz,x;bar=baz;aa=bb)`},
			pathExpression: "medianSeries(foo;bar=baz,x;bar=baz;aa=bb)",
		},
	})
	f(`groupByTags(
		group(
			time("foo;bar=baz", 30),
			time("x;bar=baz;aa=bb", 30)
		),
		'median',
		'bar', 'name'
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "foo;bar=baz",
			Tags:           map[string]string{"aggregatedBy": "median", "bar": "baz", "name": "foo"},
			pathExpression: "foo",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "x;bar=baz",
			Tags:           map[string]string{"aa": "bb", "aggregatedBy": "median", "bar": "baz", "name": "x"},
			pathExpression: "x",
		},
	})
	f(`highest(
		group(
			time('foo',25),
			time('bar',27),
			time('baz',23),
		)
	)`, []*series{
		{
			Timestamps: []int64{120000, 147000, 174000, 201000},
			Values:     []float64{120, 147, 174, 201},
			Name:       "bar",
			Tags:       map[string]string{"name": "bar"},
		},
	})
	f(`highest(
		group(
			time('foo',25),
			time('bar',27),
			time('baz',23),
		),
		4,
		'avg'
	)`, []*series{
		{
			Timestamps: []int64{120000, 147000, 174000, 201000},
			Values:     []float64{120, 147, 174, 201},
			Name:       "bar",
			Tags:       map[string]string{"name": "bar"},
		},
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{120, 145, 170, 195},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps: []int64{120000, 143000, 166000, 189000},
			Values:     []float64{120, 143, 166, 189},
			Name:       "baz",
			Tags:       map[string]string{"name": "baz"},
		},
	})
	f(`highestAverage(
		group(
			time('foo',25),
			time('bar',27),
			time('baz',23),
		),
		1
	)`, []*series{
		{
			Timestamps: []int64{120000, 147000, 174000, 201000},
			Values:     []float64{120, 147, 174, 201},
			Name:       "bar",
			Tags:       map[string]string{"name": "bar"},
		},
	})
	f(`highestCurrent(
		group(
			time('foo',25),
			time('bar',27),
			time('baz',23),
		),
		1
	)`, []*series{
		{
			Timestamps: []int64{120000, 147000, 174000, 201000},
			Values:     []float64{120, 147, 174, 201},
			Name:       "bar",
			Tags:       map[string]string{"name": "bar"},
		},
	})
	f(`highestMax(
		group(
			time('foo',25),
			time('bar',27),
			time('baz',23),
		),
		2
	)`, []*series{
		{
			Timestamps: []int64{120000, 147000, 174000, 201000},
			Values:     []float64{120, 147, 174, 201},
			Name:       "bar",
			Tags:       map[string]string{"name": "bar"},
		},
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{120, 145, 170, 195},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`hitcount(time('foo',20),'60s')`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{6000, 4000},
			Name:       "hitcount(foo,'60s')",
			Tags:       map[string]string{"name": "foo", "hitcount": "60s"},
		},
	})
	f(`hitcount(time('foo',25),'60s')`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{7875, 5475},
			Name:       "hitcount(foo,'60s')",
			Tags:       map[string]string{"name": "foo", "hitcount": "60s"},
		},
	})
	f(`hitcount(time('foo',25),'60s',true)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{7875, 5475},
			Name:       "hitcount(foo,'60s',true)",
			Tags:       map[string]string{"name": "foo", "hitcount": "60s"},
		},
	})
	f(`identity('foo')`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`integral(
		group(
			time('foo',30),
			time('bar',25),
		)
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{120, 270, 450, 660},
			Name:       "integral(foo)",
			Tags:       map[string]string{"name": "foo", "integral": "1"},
		},
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{120, 265, 435, 630},
			Name:       "integral(bar)",
			Tags:       map[string]string{"name": "bar", "integral": "1"},
		},
	})
	f(`integralByInterval(
		group(
			time('foo',30),
			time('bar',25),
		),
		'60s'
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{120, 270, 180, 390},
			Name:       "integralByInterval(foo,'60s')",
			Tags:       map[string]string{"name": "foo", "integralByInterval": "1"},
		},
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{120, 265, 435, 195},
			Name:       "integralByInterval(bar,'60s')",
			Tags:       map[string]string{"name": "bar", "integralByInterval": "1"},
		},
	})
	f(`interpolate(time('a'))`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, 180},
			Name:           "interpolate(a)",
			Tags:           map[string]string{"name": "a"},
			pathExpression: "interpolate(a)",
		},
	})
	f(`invert(time('a'))`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{0.008333333333333333, 0.005555555555555556},
			Name:           "invert(a)",
			Tags:           map[string]string{"name": "a", "invert": "1"},
			pathExpression: "invert(a)",
		},
	})
	f(`keepLastValue(removeAboveValue(time('a'),150))`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, 120},
			Name:           "keepLastValue(removeAboveValue(a,150))",
			Tags:           map[string]string{"name": "a"},
			pathExpression: ("keepLastValue(removeAboveValue(a,150))"),
		},
	})
	f(`limit(
		group(
			time('foo',25),
			time('bar',27),
			time('baz',23),
		),
		1
	)`, []*series{
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{120, 145, 170, 195},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`lineWidth(time('a'),2)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "a",
			Tags:       map[string]string{"name": "a"},
		},
	})
	f(`logarithm(time('a'))`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{2.0791812460476247, 2.255272505103306},
			Name:       "log(a,10)",
			Tags:       map[string]string{"name": "a", "log": "10"},
		},
	})
	f(`logarithm(time('a'), 2)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{6.906890595608519, 7.491853096329675},
			Name:       "log(a,2)",
			Tags:       map[string]string{"name": "a", "log": "2"},
		},
	})
	f(`logarithm(time('a'),-2)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{nan, nan},
			Name:       "log(a,-2)",
			Tags:       map[string]string{"name": "a", "log": "-2"},
		},
	})
	f(`logit(invert(time('a')))`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{-4.77912349311153, -5.187385805840755},
			Name:           "logit(invert(a))",
			Tags:           map[string]string{"name": "a", "invert": "1", "logit": "logit"},
			pathExpression: "logit(invert(a))",
		},
	})
	f(`logit(time('a'))`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{nan, nan},
			Name:           "logit(a)",
			Tags:           map[string]string{"name": "a", "logit": "logit"},
			pathExpression: "logit(a)",
		},
	})
	f(`lowest(
		group(
			time('foo',25),
			time('bar',27),
			time('baz',23),
		)
	)`, []*series{
		{
			Timestamps: []int64{120000, 143000, 166000, 189000},
			Values:     []float64{120, 143, 166, 189},
			Name:       "baz",
			Tags:       map[string]string{"name": "baz"},
		},
	})
	f(`lowest(
		group(
			time('foo',25),
			time('bar',27),
			time('baz',23),
		),
		2,
		'sum'
	)`, []*series{
		{
			Timestamps: []int64{120000, 143000, 166000, 189000},
			Values:     []float64{120, 143, 166, 189},
			Name:       "baz",
			Tags:       map[string]string{"name": "baz"},
		},
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{120, 145, 170, 195},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`lowestAverage(
		group(
			time('foo',25),
			time('bar',27),
			time('baz',23),
		),
		1
	)`, []*series{
		{
			Timestamps: []int64{120000, 143000, 166000, 189000},
			Values:     []float64{120, 143, 166, 189},
			Name:       "baz",
			Tags:       map[string]string{"name": "baz"},
		},
	})
	f(`lowestCurrent(
		group(
			time('foo',25),
			time('bar',27),
			time('baz',23),
		),
		1
	)`, []*series{
		{
			Timestamps: []int64{120000, 143000, 166000, 189000},
			Values:     []float64{120, 143, 166, 189},
			Name:       "baz",
			Tags:       map[string]string{"name": "baz"},
		},
	})
	f(`maxSeries(
		time('foo',30),
		time('bar',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "maxSeries(bar,foo)",
			Tags:       map[string]string{"name": "maxSeries(bar,foo)", "aggregatedBy": "max"},
		},
	})
	f(`maximumAbove(
		group(
			time('foo'),
			constantLine(10)|alias('bar'),
			time('baz', 20)|add(100),
		),
		200
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{220, 240, 260, 280, 300},
			Name:       "add(baz,100)",
			Tags:       map[string]string{"name": "baz", "add": "100"},
		},
	})
	f(`maximumBelow(
		group(
			time('foo'),
			constantLine(10)|alias('bar'),
			time('baz', 20)|add(100),
		),
		200
	)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps:     []int64{120000, 165000, 210000},
			Values:         []float64{10, 10, 10},
			Name:           "bar",
			Tags:           map[string]string{"name": "10"},
			pathExpression: "constantLine(10)",
		},
	})
	f(`minMax(time('foo',20))`, []*series{
		{
			Timestamps:     []int64{120000, 140000, 160000, 180000, 200000},
			Values:         []float64{0, 0.25, 0.5, 0.75, 1},
			Name:           "minMax(foo)",
			Tags:           map[string]string{"name": "foo"},
			pathExpression: "minMax(foo)",
		},
	})
	f(`minSeries(
		time('foo',30),
		time('bar',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "minSeries(bar,foo)",
			Tags:       map[string]string{"name": "minSeries(bar,foo)", "aggregatedBy": "min"},
		},
	})
	f(`minimumAbove(
		group(
			time('foo'),
			constantLine(10)|alias('bar'),
			time('baz', 20)|add(100),
		),
		200
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{220, 240, 260, 280, 300},
			Name:       "add(baz,100)",
			Tags:       map[string]string{"name": "baz", "add": "100"},
		},
	})
	f(`minimumBelow(
		group(
			time('foo'),
			constantLine(10)|alias('bar'),
			time('baz', 20)|add(100),
		),
		200
	)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps:     []int64{120000, 165000, 210000},
			Values:         []float64{10, 10, 10},
			Name:           "bar",
			Tags:           map[string]string{"name": "10"},
			pathExpression: "constantLine(10)",
		},
	})
	f(`mostDeviant(
		group(
			time('foo',18),
			time('bar',23),
			time('baz',30),
		),
		1
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{120, 150, 180, 210},
			Name:       "baz",
			Tags:       map[string]string{"name": "baz"},
		},
	})
	f(`movingAverage(
		group(
			time('foo',30),
			time('bar',30),
		),
		5
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{30, 60, 90, 120},
			Name:       "movingAverage(foo,5)",
			Tags:       map[string]string{"name": "foo", "movingAverage": "5"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{30, 60, 90, 120},
			Name:       "movingAverage(bar,5)",
			Tags:       map[string]string{"name": "bar", "movingAverage": "5"},
		},
	})
	f(`movingAverage(
		summarize(
			group(
				time('foo',10),
				time('bar',20),
			),'1m','sum',false
		),
		2
	)`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{330, 690},
			Name:       "movingAverage(summarize(foo,'1m','sum'),2)",
			Tags:       map[string]string{"name": "foo", "movingAverage": "2", "summarize": "1m", "summarizeFunction": "sum"},
		},
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{150, 330},
			Name:       "movingAverage(summarize(bar,'1m','sum'),2)",
			Tags:       map[string]string{"name": "bar", "movingAverage": "2", "summarize": "1m", "summarizeFunction": "sum"},
		},
	})
	f(`movingMax(
		group(
			time('foo',30),
			time('bar',30),
		),
		5
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{90, 120, 150, 180},
			Name:       "movingMax(foo,5)",
			Tags:       map[string]string{"name": "foo", "movingMax": "5"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{90, 120, 150, 180},
			Name:       "movingMax(bar,5)",
			Tags:       map[string]string{"name": "bar", "movingMax": "5"},
		},
	})
	f(`movingMedian(
		group(
			time('foo',30),
			time('bar',30),
		),
		5
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{30, 60, 90, 120},
			Name:       "movingMedian(foo,5)",
			Tags:       map[string]string{"name": "foo", "movingMedian": "5"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{30, 60, 90, 120},
			Name:       "movingMedian(bar,5)",
			Tags:       map[string]string{"name": "bar", "movingMedian": "5"},
		},
	})
	f(`movingMin(
		group(
			time('foo',30),
			time('bar',30),
		),
		5
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{-30, 0, 30, 60},
			Name:       "movingMin(foo,5)",
			Tags:       map[string]string{"name": "foo", "movingMin": "5"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{-30, 0, 30, 60},
			Name:       "movingMin(bar,5)",
			Tags:       map[string]string{"name": "bar", "movingMin": "5"},
		},
	})
	f(`movingSum(
		group(
			time('foo',30),
			time('bar',30),
		),
		5
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{150, 300, 450, 600},
			Name:       "movingSum(foo,5)",
			Tags:       map[string]string{"name": "foo", "movingSum": "5"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{150, 300, 450, 600},
			Name:       "movingSum(bar,5)",
			Tags:       map[string]string{"name": "bar", "movingSum": "5"},
		},
	})
	f(`movingWindow(
		group(
			time('foo',30),
			time('bar',30),
		),
		5
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{30, 60, 90, 120},
			Name:       "movingAvg(foo,5)",
			Tags:       map[string]string{"name": "foo", "movingAvg": "5"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{30, 60, 90, 120},
			Name:       "movingAvg(bar,5)",
			Tags:       map[string]string{"name": "bar", "movingAvg": "5"},
		},
	})
	f(`movingWindow(
		group(
			time('foo',30),
			time('bar',30),
		),
		'30s',
		'avg_zero'
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{90, 120, 150, 180},
			Name:       `movingAvg_zero(foo,'30s')`,
			Tags:       map[string]string{"name": "foo", "movingAvg_zero": "'30s'"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{90, 120, 150, 180},
			Name:       `movingAvg_zero(bar,'30s')`,
			Tags:       map[string]string{"name": "bar", "movingAvg_zero": "'30s'"},
		},
	})
	f(`multiplySeries(
		time('foo',30),
		time('bar',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{14400, 22500, 32400},
			Name:       "multiplySeries(bar,foo)",
			Tags:       map[string]string{"name": "multiplySeries(bar,foo)", "aggregatedBy": "multiply"},
		},
	})
	f(`multiplySeriesWithWildcards(
		group(
			time('foo.bar',30),
			time('foo.baz',30),
			time('xxx.yy',30),
		),
		1
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "xxx",
			Tags:           map[string]string{"aggregatedBy": "multiply", "name": "xxx.yy"},
			pathExpression: "xxx.yy",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{14400, 22500, 32400},
			Name:           "foo",
			Tags:           map[string]string{"aggregatedBy": "multiply", "name": "multiplySeries(foo.bar,foo.baz)"},
			pathExpression: "multiplySeries(foo.bar,foo.baz)",
		},
	})
	f(`nPercentile(
		group(
			time('a',20),
			time('b',17)
		),
		30
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{140, 140, 140, 140, 140},
			Name:       "nPercentile(a,30)",
			Tags:       map[string]string{"name": "a", "nPercentile": "30"},
		},
		{
			Timestamps: []int64{120000, 137000, 154000, 171000, 188000, 205000},
			Values:     []float64{154, 154, 154, 154, 154, 154},
			Name:       "nPercentile(b,30)",
			Tags:       map[string]string{"name": "b", "nPercentile": "30"},
		},
	})
	f(`nonNegativeDerivative(time('foo.bar;baz=1',25))`, []*series{
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{nan, 25, 25, 25},
			Name:       "nonNegativeDerivative(foo.bar;baz=1)",
			Tags:       map[string]string{"name": "foo.bar", "baz": "1", "nonNegativeDerivative": "1"},
		},
	})
	f(`offset(time('a'),10)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{130, 190},
			Name:           "offset(a,10)",
			Tags:           map[string]string{"name": "a", "offset": "10"},
			pathExpression: "offset(a,10)",
		},
	})
	f(`offsetToZero(time('a',30))`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000, 210000},
			Values:         []float64{0, 30, 60, 90},
			Name:           "offsetToZero(a)",
			Tags:           map[string]string{"name": "a", "offsetToZero": "120"},
			pathExpression: "offsetToZero(a)",
		},
	})
	f(`rangeOfSeries(
		time('foo',30),
		time('bar',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{0, 0, 0},
			Name:       "rangeOfSeries(bar,foo)",
			Tags:       map[string]string{"name": "rangeOfSeries(bar,foo)", "aggregatedBy": "rangeOf"},
		},
	})
	f(`pow(time('a'),0.5)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{10.954451150103322, 13.416407864998739},
			Name:           "pow(a,0.5)",
			Tags:           map[string]string{"name": "a", "pow": "0.5"},
			pathExpression: "pow(a,0.5)",
		},
	})
	f(`powSeries(
		time('a',30),
		time('b',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{3.1750423737803376e+249, math.Inf(1), math.Inf(1)},
			Name:       "powSeries(a,b)",
			Tags:       map[string]string{"name": "powSeries(a,b)", "aggregatedBy": "pow"},
		},
	})
	f(`removeAbovePercentile(time('a',35), 50)`, []*series{
		{
			Timestamps:     []int64{120000, 155000, 190000},
			Values:         []float64{120, 155, nan},
			Name:           "removeAbovePercentile(a,50)",
			Tags:           map[string]string{"name": "a"},
			pathExpression: "removeAbovePercentile(a,50)",
		},
	})
	f(`removeAboveValue(time('a'), 150)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, nan},
			Name:           "removeAboveValue(a,150)",
			Tags:           map[string]string{"name": "a"},
			pathExpression: "removeAboveValue(a,150)",
		},
	})
	f(`removeBelowPercentile(time('a'), 50)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{nan, 180},
			Name:           "removeBelowPercentile(a,50)",
			Tags:           map[string]string{"name": "a"},
			pathExpression: "removeBelowPercentile(a,50)",
		},
	})
	f(`removeBelowValue(time('a'), 150)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{nan, 180},
			Name:           "removeBelowValue(a,150)",
			Tags:           map[string]string{"name": "a"},
			pathExpression: "removeBelowValue(a,150)",
		},
	})
	f(`removeBetweenPercentile(
		group(
			time('a',30),
			time('b',30),
			time('c',30),
		),
		70
	)`, []*series{})
	f(`removeEmptySeries(time('a'))`, []*series{
		{
			Timestamps: []int64{120000, 180000},
			Values:     []float64{120, 180},
			Name:       "a",
			Tags:       map[string]string{"name": "a"},
		},
	})
	f(`removeEmptySeries(removeBelowValue(time('a'),150),1)`, []*series{})
	f(`round(time('a',17),-1)`, []*series{
		{
			Timestamps:     []int64{120000, 137000, 154000, 171000, 188000, 205000},
			Values:         []float64{120, 140, 150, 170, 190, 210},
			Name:           "round(a,-1)",
			Tags:           map[string]string{"name": "a"},
			pathExpression: "round(a,-1)",
		},
	})
	f(`round(time('a',17))`, []*series{
		{
			Timestamps:     []int64{120000, 137000, 154000, 171000, 188000, 205000},
			Values:         []float64{120, 137, 154, 171, 188, 205},
			Name:           "round(a)",
			Tags:           map[string]string{"name": "a"},
			pathExpression: "round(a)",
		},
	})
	f(`scale(time('a'),0.5)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{60, 90},
			Name:           "scale(a,0.5)",
			Tags:           map[string]string{"name": "a"},
			pathExpression: ("scale(a,0.5)"),
		},
	})
	f(`setXFilesFactor(
		time('foo',20),
		0.5
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{120, 140, 160, 180, 200},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo", "xFilesFactor": "0.5"},
		},
	})
	f(`sumSeriesWithWildcards(
		group(
			time('foo.bar',30),
			time('foo.baz',30),
			time('xxx.yy',30),
		),
		1
	)`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "xxx",
			Tags:           map[string]string{"aggregatedBy": "sum", "name": "xxx.yy"},
			pathExpression: "xxx.yy",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{240, 300, 360},
			Name:           "foo",
			Tags:           map[string]string{"aggregatedBy": "sum", "name": "sumSeries(foo.bar,foo.baz)"},
			pathExpression: "sumSeries(foo.bar,foo.baz)",
		},
	})
	f(`summarize(
		group(
			time('foo',13),
			time('bar',21),
		),
		'45s'
	)`, []*series{
		{
			Timestamps: []int64{90000, 135000, 180000},
			Values:     []float64{333, 327, 411},
			Name:       `summarize(bar,'45s','sum')`,
			Tags:       map[string]string{"name": "bar", "summarize": "45s", "summarizeFunction": "sum"},
		},
		{
			Timestamps: []int64{90000, 135000, 180000},
			Values:     []float64{438, 465, 802},
			Name:       `summarize(foo,'45s','sum')`,
			Tags:       map[string]string{"name": "foo", "summarize": "45s", "summarizeFunction": "sum"},
		},
	})
	f(`summarize(
		group(
			time('foo',13),
			time('bar',21),
		),
		'45s',
		'sum',
		True
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{558, 555},
			Name:       `summarize(foo,'45s','sum',true)`,
			Tags:       map[string]string{"name": "foo", "summarize": "45s", "summarizeFunction": "sum"},
		},
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{423, 387},
			Name:       `summarize(bar,'45s','sum',true)`,
			Tags:       map[string]string{"name": "bar", "summarize": "45s", "summarizeFunction": "sum"},
		},
	})
	f(`summarize(
		group(
			time('foo',13),
			time('bar',21),
		),
		'45s',
		'sumSeries',
		True
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{558, 555},
			Name:       `summarize(foo,'45s','sumSeries',true)`,
			Tags:       map[string]string{"name": "foo", "summarize": "45s", "summarizeFunction": "sumSeries"},
		},
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{423, 387},
			Name:       `summarize(bar,'45s','sumSeries',true)`,
			Tags:       map[string]string{"name": "bar", "summarize": "45s", "summarizeFunction": "sumSeries"},
		},
	})
	f(`summarize(
		group(
			time('foo',13),
			time('bar',21),
		),
		'45s',
		'last',
		True
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{159, 198},
			Name:       `summarize(foo,'45s','last',true)`,
			Tags:       map[string]string{"name": "foo", "summarize": "45s", "summarizeFunction": "last"},
		},
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{162, 204},
			Name:       `summarize(bar,'45s','last',true)`,
			Tags:       map[string]string{"name": "bar", "summarize": "45s", "summarizeFunction": "last"},
		},
	})
	f(`time('foo.bar;baz=aa', 40)`, []*series{
		{
			Timestamps:     []int64{ec.startTime, ec.startTime + 40e3, ec.startTime + 80e3},
			Values:         []float64{float64(ec.startTime) / 1e3, float64(ec.startTime)/1e3 + 40, float64(ec.startTime)/1e3 + 80},
			Name:           "foo.bar;baz=aa",
			Tags:           map[string]string{"name": "foo.bar", "baz": "aa"},
			pathExpression: "foo.bar;baz=aa",
		},
	})
	f(`timeFunction("foo.bar.baz")`, []*series{
		{
			Timestamps:     []int64{ec.startTime, ec.startTime + 60e3},
			Values:         []float64{float64(ec.startTime) / 1e3, float64(ec.startTime)/1e3 + 60},
			Name:           "foo.bar.baz",
			Tags:           map[string]string{"name": "foo.bar.baz"},
			pathExpression: "foo.bar.baz",
		},
	})
	f(`timeFunction('foo.bar;baz=aa', step=30)`, []*series{
		{
			Timestamps: []int64{ec.startTime, ec.startTime + 30e3, ec.startTime + 60e3, ec.startTime + 90e3},
			Values:     []float64{float64(ec.startTime) / 1e3, float64(ec.startTime)/1e3 + 30, float64(ec.startTime)/1e3 + 60, float64(ec.startTime)/1e3 + 90},
			Name:       "foo.bar;baz=aa",
			Tags:       map[string]string{"name": "foo.bar", "baz": "aa"},
		},
	})
	f(`weightedAverage(time('foo',30),time('bar',30))`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "weightedAverage(foo,bar,)",
			Tags:       map[string]string{"name": "weightedAverage(foo,bar,)"},
		},
	})
	f(`weightedAverage(
		group(
			time("foo.x", 30),
			time("bar.y", 30),
		),
		group(
			time("bar.x", 30),
			time("foo.y", 30),
		),
		0
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "weightedAverage(bar.y,foo.x,bar.x,foo.y,0)",
			Tags:       map[string]string{"name": "weightedAverage(bar.y,foo.x,bar.x,foo.y,0)"},
		},
	})
	f(`weightedAverage(
		group(
			time("foo", 10) | alias("foo.bar1"),
			time("bar", 10) | alias("foo.bar2"),
		),
		group(
			time("bar", 10) | alias("foo.bar3"),
			time("foo", 10) | alias("foo.bar4"),
		),
        1
	)`, []*series{})
	f(`weightedAverage(
		group(
			time("foo0.bar2",30),
			time("foo0.bar1",30) ,
		),
		group(
			time("foo1.bar1",30),
			time("foo1.bar2",30),
		),
        1
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "weightedAverage(foo0.bar1,foo0.bar2,foo1.bar1,foo1.bar2,1)",
			Tags:       map[string]string{"name": "weightedAverage(foo0.bar1,foo0.bar2,foo1.bar1,foo1.bar2,1)"},
		},
	})
	f(`xFilesFactor(
		time('foo',20),
		0.5
	)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{120, 140, 160, 180, 200},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo", "xFilesFactor": "0.5"},
		},
	})
	f(`verticalLine("00:03_19700101","event","blue")`, []*series{
		{
			Timestamps: []int64{180000, 180000},
			Values:     []float64{1.0, 1.0},
			Name:       "event",
			Tags:       map[string]string{"name": "event"},
		},
	})
	f(`verticalLine("00:0319700101","event","blue")`, []*series{
		{
			Timestamps: []int64{180000, 180000},
			Values:     []float64{1.0, 1.0},
			Name:       "event",
			Tags:       map[string]string{"name": "event"},
		},
	})
	f(`useSeriesAbove(time('foo.baz',10),10000,"reqs","time")`, []*series{})
	f(`unique(time('foo',30),time('foo',30))`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{120.0, 150.0, 180.0, 210.0},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
	})
	f(`unique(time('foo',30),time('foo',40),time('foo.bar',40))`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000, 210000},
			Values:     []float64{120.0, 150.0, 180.0, 210.0},
			Name:       "foo",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps: []int64{120000, 160000, 200000},
			Values:     []float64{120.0, 160.0, 200.0},
			Name:       "foo.bar",
			Tags:       map[string]string{"name": "foo.bar"},
		},
	})
	f(`perSecond(time('foo.bar;baz=1',25))`, []*series{
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{nan, 1, 1, 1},
			Name:       "perSecond(foo.bar;baz=1)",
			Tags:       map[string]string{"name": "foo.bar", "baz": "1", "perSecond": "1"},
		},
	})
	f(`perSecond(time('foo.bar;baz=1',25),150)`, []*series{
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{nan, 1, nan, nan},
			Name:       "perSecond(foo.bar;baz=1)",
			Tags:       map[string]string{"name": "foo.bar", "baz": "1", "perSecond": "1"},
		},
	})
	f(`perSecond(time('foo.bar;baz=1',25),None,140)`, []*series{
		{
			Timestamps: []int64{120000, 145000, 170000, 195000},
			Values:     []float64{nan, nan, 1, 1},
			Name:       "perSecond(foo.bar;baz=1)",
			Tags:       map[string]string{"name": "foo.bar", "baz": "1", "perSecond": "1"},
		},
	})
	f(`percentileOfSeries(
		group(
			time('a',30),
			time('b',30),
			time('c',30)
		),
		40
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "percentileOfSeries(a,40)",
			Tags:       map[string]string{"name": "percentileOfSeries(a,40)"},
		},
	})
	f(`percentileOfSeries(
		group(
			time('a',30),
			time('b',30),
			time('c',30),
		),
		90
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "percentileOfSeries(a,90)",
			Tags:       map[string]string{"name": "percentileOfSeries(a,90)"},
		},
	})
	f(`transformNull(time('foo.bar',35),-1,time('foo.bar',30))`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 155, 190},
			Name:           "transformNull(foo.bar,-1,referenceSeries)",
			Tags:           map[string]string{"name": "foo.bar", "referenceSeries": "1", "transformNull": "-1"},
			pathExpression: "transformNull(foo.bar,-1,referenceSeries)",
		},
	})
	f(`transformNull(time('foo.bar',35),-1)`, []*series{
		{
			Timestamps:     []int64{120000, 155000, 190000},
			Values:         []float64{120, 155, 190},
			Name:           "transformNull(foo.bar,-1)",
			Tags:           map[string]string{"name": "foo.bar", "transformNull": "-1"},
			pathExpression: "transformNull(foo.bar,-1)",
		},
	})
	f(`timeShift(time('foo.bar;baz=1',25),"+1min")`, []*series{
		{
			Timestamps:     []int64{120000, 145000},
			Values:         []float64{180, 205},
			Name:           `timeShift(foo.bar;baz=1,'+1min')`,
			Tags:           map[string]string{"name": "foo.bar", "baz": "1", "timeShift": "+1min"},
			pathExpression: "foo.bar;baz=1",
		},
	})
	f(`timeShift(time('foo.bar;baz=1',25),"+1min",false)`, []*series{
		{
			Timestamps:     []int64{120000, 145000, 170000, 195000},
			Values:         []float64{180, 205, 230, 255},
			Name:           `timeShift(foo.bar;baz=1,'+1min')`,
			Tags:           map[string]string{"name": "foo.bar", "baz": "1", "timeShift": "+1min"},
			pathExpression: "foo.bar;baz=1",
		},
	})
	f(`timeShift(time('foo.bar;baz=1',25),"-1min",true)`, []*series{
		{
			Timestamps:     []int64{120000, 145000, 170000, 195000},
			Values:         []float64{60, 85, 110, 135},
			Name:           "timeShift(foo.bar;baz=1,'-1min')",
			Tags:           map[string]string{"name": "foo.bar", "baz": "1", "timeShift": "-1min"},
			pathExpression: "foo.bar;baz=1",
		},
	})
	f(`timeShift(time('foo.bar;baz=1',25),"1min",false,true)`, []*series{
		{
			Timestamps:     []int64{120000, 145000, 170000, 195000},
			Values:         []float64{60, 85, 110, 135},
			Name:           `timeShift(foo.bar;baz=1,'1min')`,
			Tags:           map[string]string{"name": "foo.bar", "baz": "1", "timeShift": "1min"},
			pathExpression: "foo.bar;baz=1",
		},
	})
	f(`timeSlice(time('foo.bar;bar=1',20),"00:00 19700101","00:03 19700101")`, []*series{
		{
			Timestamps:     []int64{120000, 140000, 160000, 180000, 200000},
			Values:         []float64{120, 140, 160, 180, nan},
			Name:           "timeSlice(foo.bar;bar=1,0,180)",
			Tags:           map[string]string{"name": "foo.bar", "bar": "1", "timeSliceEnd": "180", "timeSliceStart": "0"},
			pathExpression: "foo.bar;bar=1",
		},
	})
	f(`timeStack(time('foo.bar',35),"+1min",1,3)`, []*series{
		{
			Timestamps:     []int64{120000, 155000, 190000},
			Values:         []float64{180, 215, 250},
			Name:           "timeShift(foo.bar,+1min,1)",
			Tags:           map[string]string{"name": "foo.bar", "timeShift": "1", "timeShiftUnit": "+1min"},
			pathExpression: `timeShift(foo.bar,+1min,1)`,
		},
		{
			Timestamps:     []int64{120000, 155000, 190000},
			Values:         []float64{240, 275, 310},
			Name:           "timeShift(foo.bar,+1min,2)",
			Tags:           map[string]string{"name": "foo.bar", "timeShift": "2", "timeShiftUnit": "+1min"},
			pathExpression: `timeShift(foo.bar,+1min,2)`,
		},
		{
			Timestamps:     []int64{120000, 155000, 190000},
			Values:         []float64{300, 335, 370},
			Name:           "timeShift(foo.bar,+1min,3)",
			Tags:           map[string]string{"name": "foo.bar", "timeShift": "3", "timeShiftUnit": "+1min"},
			pathExpression: `timeShift(foo.bar,+1min,3)`,
		},
	})
	f(`timeStack(time('foo.bar',35),"1min",1,3)`, []*series{
		{
			Timestamps:     []int64{120000, 155000, 190000},
			Values:         []float64{60, 95, 130},
			Name:           "timeShift(foo.bar,1min,1)",
			Tags:           map[string]string{"name": "foo.bar", "timeShift": "1", "timeShiftUnit": "1min"},
			pathExpression: `timeShift(foo.bar,1min,1)`,
		},
		{
			Timestamps:     []int64{120000, 155000, 190000},
			Values:         []float64{0, 35, 70},
			Name:           "timeShift(foo.bar,1min,2)",
			Tags:           map[string]string{"name": "foo.bar", "timeShift": "2", "timeShiftUnit": "1min"},
			pathExpression: `timeShift(foo.bar,1min,2)`,
		},
		{
			Timestamps:     []int64{120000, 155000, 190000},
			Values:         []float64{-60, -25, 10},
			Name:           "timeShift(foo.bar,1min,3)",
			Tags:           map[string]string{"name": "foo.bar", "timeShift": "3", "timeShiftUnit": "1min"},
			pathExpression: `timeShift(foo.bar,1min,3)`,
		},
	})
	f(`threshold(1.5)`, []*series{
		{
			Timestamps:     []int64{120000, 165000, 210000},
			Values:         []float64{1.5, 1.5, 1.5},
			Name:           "1.5",
			Tags:           map[string]string{"name": "1.5"},
			pathExpression: "threshold(1.5)",
		},
	})
	f(`threshold(1.5,"max","black")`, []*series{
		{
			Timestamps:     []int64{120000, 165000, 210000},
			Values:         []float64{1.5, 1.5, 1.5},
			Name:           "max",
			Tags:           map[string]string{"name": "1.5"},
			pathExpression: "threshold(1.5,'max','black')",
		},
	})
	f(`sum(
		time('foo',30),
		time('bar',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{240, 300, 360},
			Name:       "sumSeries(bar,foo)",
			Tags:       map[string]string{"name": "sumSeries(bar,foo)", "aggregatedBy": "sum"},
		},
	})
	f(`sumSeries(
		time('foo',30),
		time('bar',30),
	)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{240, 300, 360},
			Name:       "sumSeries(bar,foo)",
			Tags:       map[string]string{"name": "sumSeries(bar,foo)", "aggregatedBy": "sum"},
		},
	})
	f(`substr(time('collectd.test-db1.load.value;tag1=value1;tag2=value2'),1,3)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, 180},
			Name:           "test-db1.load",
			Tags:           map[string]string{"name": "collectd.test-db1.load.value", "tag1": "value1", "tag2": "value2"},
			pathExpression: "collectd.test-db1.load.value;tag1=value1;tag2=value2",
		},
	})

	f(`substr(time('foo.baz.host;tag1=value1;tag2=value2'),1)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, 180},
			Name:           "baz.host;tag1=value1;tag2=value2",
			Tags:           map[string]string{"name": "foo.baz.host", "tag1": "value1", "tag2": "value2"},
			pathExpression: "foo.baz.host;tag1=value1;tag2=value2",
		},
	})

	f(`substr(time('foo.baz.host;tag1=value1;tag2=value2'),5)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, 180},
			Name:           "",
			Tags:           map[string]string{"name": "foo.baz.host", "tag1": "value1", "tag2": "value2"},
			pathExpression: "foo.baz.host;tag1=value1;tag2=value2",
		},
	})
	f(`substr(time('foo.baz.host;tag1=value1;tag2=value2'),1,10)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, 180},
			Name:           "baz.host;tag1=value1;tag2=value2",
			Tags:           map[string]string{"name": "foo.baz.host", "tag1": "value1", "tag2": "value2"},
			pathExpression: "foo.baz.host;tag1=value1;tag2=value2",
		},
	})
	f(`substr(time('foo.baz.host;tag1=value1;tag2=value2'),-1)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, 180},
			Name:           "host;tag1=value1;tag2=value2",
			Tags:           map[string]string{"name": "foo.baz.host", "tag1": "value1", "tag2": "value2"},
			pathExpression: "foo.baz.host;tag1=value1;tag2=value2",
		},
	})
	f(`substr(time('foo.baz.host;tag1=value1;tag2=value2'),1,-1)`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{120, 180},
			Name:           "baz",
			Tags:           map[string]string{"name": "foo.baz.host", "tag1": "value1", "tag2": "value2"},
			pathExpression: "foo.baz.host;tag1=value1;tag2=value2",
		},
	})
	f(`stdev(time('foo.baz',20),3,0.1)`, []*series{
		{
			Timestamps:     []int64{120000, 140000, 160000, 180000, 200000},
			Values:         []float64{0, 10, 16.32993161855452, 16.32993161855452, 16.32993161855452},
			Name:           "stdev(foo.baz,3)",
			Tags:           map[string]string{"name": "foo.baz", "stdev": "3"},
			pathExpression: "foo.baz",
		},
	})
	f(`stdev(time('foo.baz',20),3,0.5)`, []*series{
		{
			Timestamps:     []int64{120000, 140000, 160000, 180000, 200000},
			Values:         []float64{nan, 10, 16.32993161855452, 16.32993161855452, 16.32993161855452},
			Name:           "stdev(foo.baz,3)",
			Tags:           map[string]string{"name": "foo.baz", "stdev": "3"},
			pathExpression: "foo.baz",
		},
	})

	f(`stddevSeries(time('foo.baz',30))`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{0, 0, 0},
			Name:       "stddevSeries(foo.baz)",
			Tags:       map[string]string{"name": "foo.baz", "aggregatedBy": "stddev"},
		},
	})

	f(`stacked(
		group(
			time("foo", 30) | alias("foo1.bar2"),
			time("bar", 30) | alias("foo1.bar3")
		))`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "stacked(foo1.bar2)",
			Tags:       map[string]string{"name": "foo", "stacked": "__DEFAULT__"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{240, 300, 360},
			Name:       "stacked(foo1.bar3)",
			Tags:       map[string]string{"name": "bar", "stacked": "__DEFAULT__"},
		},
	})
	f(`stacked(
		group(
			time("bar", 30)| alias("foo1.bar1"),
			time("foo", 30) | alias("foo1.bar2"),
			time("foo", 30) | alias("foo1.bar3")
		),
		''
     )`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120, 150, 180},
			Name:       "foo1.bar1",
			Tags:       map[string]string{"name": "bar"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{240, 300, 360},
			Name:       "foo1.bar2",
			Tags:       map[string]string{"name": "foo"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{360, 450, 540},
			Name:       "foo1.bar3",
			Tags:       map[string]string{"name": "foo"},
		},
	})

	f(`squareRoot(time('foo.baz',10))`, []*series{
		{
			Timestamps:     []int64{120000, 130000, 140000, 150000, 160000, 170000, 180000, 190000, 200000, 210000},
			Values:         []float64{10.954451150103322, 11.40175425099138, 11.832159566199232, 12.24744871391589, 12.649110640673518, 13.038404810405298, 13.416407864998739, 13.784048752090222, 14.142135623730951, 14.491376746189438},
			Name:           "squareRoot(foo.baz)",
			Tags:           map[string]string{"name": "foo.baz", "squareRoot": "1"},
			pathExpression: "squareRoot(foo.baz)",
		},
	})

	f(`sortByTotal(
		group(
			time("bar", 10)| alias("foo1.bar1"),
			time("foo", 15) | alias("foo1.bar2"),
			time("foo", 30) | alias("foo1.bar3")
     ))`, []*series{
		{
			Timestamps:     []int64{120000, 130000, 140000, 150000, 160000, 170000, 180000, 190000, 200000, 210000},
			Values:         []float64{120, 130, 140, 150, 160, 170, 180, 190, 200, 210},
			Name:           "foo1.bar1",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "bar",
		},
		{
			Timestamps:     []int64{120000, 135000, 150000, 165000, 180000, 195000, 210000},
			Values:         []float64{120, 135, 150, 165, 180, 195, 210},
			Name:           "foo1.bar2",
			Tags:           map[string]string{"name": "foo"},
			pathExpression: "foo",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000, 210000},
			Values:         []float64{120, 150, 180, 210},
			Name:           "foo1.bar3",
			Tags:           map[string]string{"name": "foo"},
			pathExpression: "foo",
		},
	})

	f(`sortBy(
		group(
			time("bar", 10)| alias("foo1.bar1"),
			time("foo", 15) | alias("foo1.bar2")
             )
    )`, []*series{
		{
			Timestamps:     []int64{120000, 130000, 140000, 150000, 160000, 170000, 180000, 190000, 200000, 210000},
			Values:         []float64{120, 130, 140, 150, 160, 170, 180, 190, 200, 210},
			Name:           "foo1.bar1",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "bar",
		},
		{
			Timestamps:     []int64{120000, 135000, 150000, 165000, 180000, 195000, 210000},
			Values:         []float64{120, 135, 150, 165, 180, 195, 210},
			Name:           "foo1.bar2",
			Tags:           map[string]string{"name": "foo"},
			pathExpression: "foo",
		},
	})
	f(`sortBy(
		group(
			time("bar", 10)| alias("foo1.bar1"),
			time("foo", 15) | alias("foo1.bar2"),
        ),'average',true)`, []*series{
		{
			Timestamps:     []int64{120000, 135000, 150000, 165000, 180000, 195000, 210000},
			Values:         []float64{120, 135, 150, 165, 180, 195, 210},
			Name:           "foo1.bar2",
			Tags:           map[string]string{"name": "foo"},
			pathExpression: "foo",
		},
		{
			Timestamps:     []int64{120000, 130000, 140000, 150000, 160000, 170000, 180000, 190000, 200000, 210000},
			Values:         []float64{120, 130, 140, 150, 160, 170, 180, 190, 200, 210},
			Name:           "foo1.bar1",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "bar",
		},
	})
	f(`sortBy(
		group(
			time("bar", 10)| alias("foo1.bar1"),
			time("foo", 15) | alias("foo1.bar2"),
        ),'multiply',true)`, []*series{
		{
			Timestamps:     []int64{120000, 135000, 150000, 165000, 180000, 195000, 210000},
			Values:         []float64{120, 135, 150, 165, 180, 195, 210},
			Name:           "foo1.bar2",
			Tags:           map[string]string{"name": "foo"},
			pathExpression: "foo",
		},
		{
			Timestamps:     []int64{120000, 130000, 140000, 150000, 160000, 170000, 180000, 190000, 200000, 210000},
			Values:         []float64{120, 130, 140, 150, 160, 170, 180, 190, 200, 210},
			Name:           "foo1.bar1",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "bar",
		},
	})
	f(`sortBy(
		group(
			time("bar", 10)| alias("foo1.bar1"),
			time("foo", 15) | alias("foo1.bar2"),
        ),'diff')`, []*series{
		{
			Timestamps:     []int64{120000, 135000, 150000, 165000, 180000, 195000, 210000},
			Values:         []float64{120, 135, 150, 165, 180, 195, 210},
			Name:           "foo1.bar2",
			Tags:           map[string]string{"name": "foo"},
			pathExpression: "foo",
		},
		{
			Timestamps:     []int64{120000, 130000, 140000, 150000, 160000, 170000, 180000, 190000, 200000, 210000},
			Values:         []float64{120, 130, 140, 150, 160, 170, 180, 190, 200, 210},
			Name:           "foo1.bar1",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "bar",
		},
	})
	f(`sortByName(
		group(
			time("bar", 10)| alias("foo1.bar1"),
			time("foo", 15) | alias("foo1.bar2")
        ))`, []*series{
		{
			Timestamps:     []int64{120000, 135000, 150000, 165000, 180000, 195000, 210000},
			Values:         []float64{120, 135, 150, 165, 180, 195, 210},
			Name:           "foo1.bar2",
			Tags:           map[string]string{"name": "foo"},
			pathExpression: "foo",
		},
		{
			Timestamps:     []int64{120000, 130000, 140000, 150000, 160000, 170000, 180000, 190000, 200000, 210000},
			Values:         []float64{120, 130, 140, 150, 160, 170, 180, 190, 200, 210},
			Name:           "foo1.bar1",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "bar",
		},
	})
	f(`sortByMaxima(
		group(
			time("bar", 10)| alias("foo1.bar1"),
			constantLine( 15) | alias("foo1.bar2")
        ))`, []*series{
		{
			Timestamps:     []int64{120000, 165000, 210000},
			Values:         []float64{15, 15, 15},
			Name:           "foo1.bar2",
			Tags:           map[string]string{"name": "15"},
			pathExpression: "constantLine(15)",
		},
		{
			Timestamps:     []int64{120000, 130000, 140000, 150000, 160000, 170000, 180000, 190000, 200000, 210000},
			Values:         []float64{120, 130, 140, 150, 160, 170, 180, 190, 200, 210},
			Name:           "foo1.bar1",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "bar",
		},
	})
	f(`sortByMinima(
		group(
			time("bar", 10)| alias("foo1.bar1"),
			constantLine( 15) | alias("foo1.bar2")
        ))`, []*series{
		{
			Timestamps:     []int64{120000, 130000, 140000, 150000, 160000, 170000, 180000, 190000, 200000, 210000},
			Values:         []float64{120, 130, 140, 150, 160, 170, 180, 190, 200, 210},
			Name:           "foo1.bar1",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "bar",
		},
		{
			Timestamps:     []int64{120000, 165000, 210000},
			Values:         []float64{15, 15, 15},
			Name:           "foo1.bar2",
			Tags:           map[string]string{"name": "15"},
			pathExpression: "constantLine(15)",
		},
	})
	f(`sortByMinima(
		group(
			time("bar", 10)| alias("foo1.bar1"),
			constantLine( 0) | alias("foo1.bar2")
        ))`, []*series{
		{
			Timestamps:     []int64{120000, 130000, 140000, 150000, 160000, 170000, 180000, 190000, 200000, 210000},
			Values:         []float64{120, 130, 140, 150, 160, 170, 180, 190, 200, 210},
			Name:           "foo1.bar1",
			Tags:           map[string]string{"name": "bar"},
			pathExpression: "bar",
		},
	})
	f(`smartSummarize(
		group(
			time('foo',13),
			time('bar',21),
		),
		'45s'
	)`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{423, 387},
			Name:       `smartSummarize(bar,'45s','sum')`,
			Tags:       map[string]string{"name": "bar", "smartSummarize": "45s", "smartSummarizeFunction": "sum"},
		},
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{558, 555},
			Name:       `smartSummarize(foo,'45s','sum')`,
			Tags:       map[string]string{"name": "foo", "smartSummarize": "45s", "smartSummarizeFunction": "sum"},
		},
	})
	f(`smartSummarize(
		group(
			time('foo',13),
			time('bar',21),
		),
		'1min','sum','hour'
	)`, []*series{
		{
			Timestamps: []int64{0, 60000, 120000},
			Values:     []float64{130, 455, 598},
			Name:       `smartSummarize(foo,'1min','sum')`,
			Tags:       map[string]string{"name": "foo", "smartSummarize": "1min", "smartSummarizeFunction": "sum"},
		},
		{
			Timestamps: []int64{0, 60000, 120000},
			Values:     []float64{63, 252, 441},
			Name:       `smartSummarize(bar,'1min','sum')`,
			Tags:       map[string]string{"name": "bar", "smartSummarize": "1min", "smartSummarizeFunction": "sum"},
		},
	})

	f(`sinFunction("base",1,30)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{0.5806111842123143, -0.7148764296291645, -0.8011526357338306},
			Name:       "base",
			Tags:       map[string]string{"name": "base"},
		},
	})
	f(`sinFunction("base",2,30)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{1.1612223684246286, -1.429752859258329, -1.602305271467661},
			Name:       "base",
			Tags:       map[string]string{"name": "base"},
		},
	})
	f(`sinFunction("base",step=20)`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{0.5806111842123143, 0.9802396594403116, 0.21942525837900473, -0.8011526357338306, -0.8732972972139945},
			Name:       "base",
			Tags:       map[string]string{"name": "base"},
		},
	})

	f(`sigmoid(time('foo.baz'))`, []*series{
		{
			Timestamps:     []int64{120000, 180000},
			Values:         []float64{1, 1},
			Name:           "sigmoid(foo.baz)",
			Tags:           map[string]string{"name": "foo.baz", "sigmoid": "sigmoid"},
			pathExpression: "sigmoid(foo.baz)",
		},
	})

	f(`scaleToSeconds(time('foo.bas',20),5)`, []*series{
		{
			Timestamps:     []int64{120000, 140000, 160000, 180000, 200000},
			Values:         []float64{30, 35, 40, 45, 50},
			Name:           "scaleToSeconds(foo.bas,5)",
			Tags:           map[string]string{"name": "foo.bas", "scaleToSeconds": "5"},
			pathExpression: "scaleToSeconds(foo.bas,5)",
		},
	})

	f(`secondYAxis(time('foo.bas',30))`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000, 210000},
			Values:         []float64{120, 150, 180, 210},
			Name:           "secondYAxis(foo.bas)",
			Tags:           map[string]string{"name": "foo.bas", "secondYAxis": "1"},
			pathExpression: "foo.bas",
		},
	})

	f(`isNonNull(timeSlice(time('foo.bar',20),"00:00 19700101","00:03 19700101"))`, []*series{
		{
			Timestamps: []int64{120000, 140000, 160000, 180000, 200000},
			Values:     []float64{1, 1, 1, 1, 0},
			Name:       "isNonNull(timeSlice(foo.bar,0,180))",
			Tags:       map[string]string{"name": "foo.bar", "isNonNull": "1", "timeSliceEnd": "180", "timeSliceStart": "0"},
		},
	})
	f(`linearRegression(
      group(
        time("foo.baz",30),
        time("baz.bar",30),
      )
     )`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "linearRegression(foo.baz, 120, 210)",
			Tags:           map[string]string{"name": "foo.baz", "linearRegressions": "120, 210"},
			pathExpression: "linearRegression(foo.baz, 120, 210)",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "linearRegression(baz.bar, 120, 210)",
			Tags:           map[string]string{"name": "baz.bar", "linearRegressions": "120, 210"},
			pathExpression: "linearRegression(baz.bar, 120, 210)",
		},
	})
	f(`linearRegression(
      group(
        time("foo.baz",30),
        time("baz.bar",30),
      ),
      startSourceAt=100,
      EndSourceAt=None,
     )`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "linearRegression(foo.baz, 100, 210)",
			Tags:           map[string]string{"name": "foo.baz", "linearRegressions": "100, 210"},
			pathExpression: "linearRegression(foo.baz, 100, 210)",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "linearRegression(baz.bar, 100, 210)",
			Tags:           map[string]string{"name": "baz.bar", "linearRegressions": "100, 210"},
			pathExpression: "linearRegression(baz.bar, 100, 210)",
		},
	})
	f(`linearRegression(
      group(
        time("foo.baz",30),
        time("baz.bar",30),
      ),
      startSourceAt=None,
      endSourceAt="00:08 19700101"
     )`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "linearRegression(foo.baz, 120, 480)",
			Tags:           map[string]string{"name": "foo.baz", "linearRegressions": "120, 480"},
			pathExpression: "linearRegression(foo.baz, 120, 480)",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           "linearRegression(baz.bar, 120, 480)",
			Tags:           map[string]string{"name": "baz.bar", "linearRegressions": "120, 480"},
			pathExpression: "linearRegression(baz.bar, 120, 480)",
		},
	})
	f(`holtWintersForecast(time("foo.baz",30))`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120.00026583823248, 151.53351196300892, 182.8503377518708},
			Name:       "holtWintersForecast(foo.baz)",
			Tags:       map[string]string{"name": "holtWintersForecast(foo.baz)", "holtWintersForecast": "1"},
		},
	})
	f(`holtWintersForecast(time("foo.baz",30),"4d")`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120.00027210295323, 152.034912932407, 183.72178095512407},
			Name:       "holtWintersForecast(foo.baz)",
			Tags:       map[string]string{"name": "holtWintersForecast(foo.baz)", "holtWintersForecast": "1"},
		},
	})
	f(`holtWintersForecast(time("foo.baz",30),"8d","2d")`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120.00000001724152, 152.03464171718454, 183.72151060765324},
			Name:       "holtWintersForecast(foo.baz)",
			Tags:       map[string]string{"name": "holtWintersForecast(foo.baz)", "holtWintersForecast": "1"},
		},
	})
	f(`holtWintersConfidenceBands(time("foo.bar",30))`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120.00214864234894, 158.95265929117159, 196.72661235783855},
			Name:       "holtWintersConfidenceUpper(foo.bar)",
			Tags:       map[string]string{"name": "foo.bar", "holtWintersConfidenceUpper": "1"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{119.99838303411602, 144.11436463484625, 168.97406314590305},
			Name:       "holtWintersConfidenceLower(foo.bar)",
			Tags:       map[string]string{"name": "foo.bar", "holtWintersConfidenceLower": "1"},
		},
	})
	f(`holtWintersConfidenceBands(time("foo.bar",30),5,"4d")`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120.00407891280487, 165.87605713703562, 209.67633502193422},
			Name:       "holtWintersConfidenceUpper(foo.bar)",
			Tags:       map[string]string{"name": "foo.bar", "holtWintersConfidenceUpper": "1"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{119.9964652931016, 138.19376872777838, 157.76722688831393},
			Name:       "holtWintersConfidenceLower(foo.bar)",
			Tags:       map[string]string{"name": "foo.bar", "holtWintersConfidenceLower": "1"},
		},
	})
	f(`holtWintersConfidenceBands(time("foo.bar",30),5,"8d","2d")`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{120.00000014163967, 165.87883899077733, 209.679106539474},
			Name:       "holtWintersConfidenceUpper(foo.bar)",
			Tags:       map[string]string{"name": "foo.bar", "holtWintersConfidenceUpper": "1"},
		},
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{119.99999989284336, 138.19044444359176, 157.7639146758325},
			Name:       "holtWintersConfidenceLower(foo.bar)",
			Tags:       map[string]string{"name": "foo.bar", "holtWintersConfidenceLower": "1"},
		},
	})

	f(`holtWintersAberration(time("baz.baf",30))`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{0, 0, 0},
			Name:       "holtWintersAberration(baz.baf)",
			Tags:       map[string]string{"name": "baz.baf", "holtWintersAberration": "1"},
		},
	})
	f(`holtWintersAberration(time("baz.baf",30),2)`, []*series{
		{
			Timestamps: []int64{120000, 150000, 180000},
			Values:     []float64{0, 0, 0},
			Name:       "holtWintersAberration(baz.baf)",
			Tags:       map[string]string{"name": "baz.baf", "holtWintersAberration": "1"},
		},
	})

	f(`holtWintersConfidenceArea(time("foo.baz",30))`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120.00214864234894, 158.95265929117159, 196.72661235783855},
			Name:           "areaBetween(holtWintersConfidenceUpper(foo.baz))",
			Tags:           map[string]string{"holtWintersConfidenceUpper": "1", "areaBetween": "1", "name": "foo.baz"},
			pathExpression: "holtWintersConfidenceUpper(foo.baz)",
		},
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{119.99838303411602, 144.11436463484625, 168.97406314590305},
			Name:           "areaBetween(holtWintersConfidenceLower(foo.baz))",
			Tags:           map[string]string{"holtWintersConfidenceLower": "1", "areaBetween": "1", "name": "foo.baz"},
			pathExpression: "holtWintersConfidenceLower(foo.baz)",
		},
	})

	f(`groupByNode(summarize(
               group(
                       time('foo.bar.baz',20),
                       time('bar.foo.bad',20),
                       time('bar.foo.bad',20),
               ),
               '45s'
       ),1)`, []*series{
		{
			Timestamps:     []int64{120000, 165000},
			Values:         []float64{325, 400},
			Name:           `foo`,
			Tags:           map[string]string{"aggregatedBy": "average", "name": "bar.foo.bad", "summarize": "45s", "summarizeFunction": "sum"},
			pathExpression: "bar.foo.bad",
		},
		{
			Timestamps:     []int64{120000, 165000},
			Values:         []float64{325, 400},
			Name:           `bar`,
			Tags:           map[string]string{"aggregatedBy": "average", "name": "foo.bar.baz", "summarize": "45s", "summarizeFunction": "sum"},
			pathExpression: "foo.bar.baz",
		},
	})
	f(`divideSeries(
    summarize(
               group(
                       time('foo.bar.baz',10),
                       time('bar.foo.bad',10)
               ),
               '45s'
       ),
    summarize(
               group(
                       time('foo.bar.baz',10)
               ),
               '45s'
       ))`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{1, 1},
			Name:       `divideSeries(summarize(foo.bar.baz,'45s','sum'),summarize(foo.bar.baz,'45s','sum'))`,
			Tags:       map[string]string{"name": "foo.bar.baz", "summarize": "45s", "summarizeFunction": "sum"},
		},
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{1, 1},
			Name:       `divideSeries(summarize(bar.foo.bad,'45s','sum'),summarize(foo.bar.baz,'45s','sum'))`,
			Tags:       map[string]string{"name": "bar.foo.bad", "summarize": "45s", "summarizeFunction": "sum"},
		},
	})
	f(`divideSeriesLists(
    summarize(
               time('foo.bar.baz',10),
               '45s'
       ),
    summarize(
               time('bar.foo.bad',10),
               '45s'
       ))`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{1, 1},
			Name:       `divideSeries(summarize(foo.bar.baz,'45s','sum'),summarize(bar.foo.bad,'45s','sum'))`,
			Tags:       map[string]string{"name": "foo.bar.baz", "summarize": "45s", "summarizeFunction": "sum"},
		},
	})
	f(`aggregateSeriesLists(
    summarize(
               time('foo.bar.baz',10),
               '45s'
       ),
    summarize(
               time('bar.foo.bad',10),
               '45s'
       ), 'sum')`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{1170, 2000},
			Name:       `sumSeries(summarize(foo.bar.baz,'45s','sum'),summarize(bar.foo.bad,'45s','sum'))`,
			Tags:       map[string]string{"name": "foo.bar.baz", "summarize": "45s", "summarizeFunction": "sum"},
		},
	})
	f(`sumSeriesLists(
    summarize(
               time('foo.bar.baz',10),
               '45s'
       ),
    summarize(
               time('bar.foo.bad',10),
               '45s'
       ))`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{1170, 2000},
			Name:       `sumSeries(summarize(foo.bar.baz,'45s','sum'),summarize(bar.foo.bad,'45s','sum'))`,
			Tags:       map[string]string{"name": "foo.bar.baz", "summarize": "45s", "summarizeFunction": "sum"},
		},
	})
	f(`aggregateSeriesLists(
    summarize(
               time('foo.bar.baz',10),
               '45s'
       ),
    summarize(
               time('bar.foo.bad',10),
               '45s'
       ), 'diff')`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{0, 0},
			Name:       `diffSeries(summarize(foo.bar.baz,'45s','sum'),summarize(bar.foo.bad,'45s','sum'))`,
			Tags:       map[string]string{"name": "foo.bar.baz", "summarize": "45s", "summarizeFunction": "sum"},
		},
	})
	f(`diffSeriesLists(
    summarize(
               time('foo.bar.baz',10),
               '45s'
       ),
    summarize(
               time('bar.foo.bad',10),
               '45s'
       ))`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{0, 0},
			Name:       `diffSeries(summarize(foo.bar.baz,'45s','sum'),summarize(bar.foo.bad,'45s','sum'))`,
			Tags:       map[string]string{"name": "foo.bar.baz", "summarize": "45s", "summarizeFunction": "sum"},
		},
	})
	f(`aggregateSeriesLists(
    summarize(
               time('foo.bar.baz',10),
               '45s'
       ),
    summarize(
               time('bar.foo.bad',10),
               '45s'
       ), 'multiply')`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{342225, 1e+06},
			Name:       `multiplySeries(summarize(foo.bar.baz,'45s','sum'),summarize(bar.foo.bad,'45s','sum'))`,
			Tags:       map[string]string{"name": "foo.bar.baz", "summarize": "45s", "summarizeFunction": "sum"},
		},
	})
	f(`multiplySeriesLists(
    summarize(
               time('foo.bar.baz',10),
               '45s'
       ),
    summarize(
               time('bar.foo.bad',10),
               '45s'
       ))`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{342225, 1e+06},
			Name:       `multiplySeries(summarize(foo.bar.baz,'45s','sum'),summarize(bar.foo.bad,'45s','sum'))`,
			Tags:       map[string]string{"name": "foo.bar.baz", "summarize": "45s", "summarizeFunction": "sum"},
		},
	})
	f(`weightedAverage(
    summarize(
               group(
                       time('foo.bar.baz',10),
                       time('bar.foo.bad',10)
               ),
               '45s'
       ),
    summarize(
               group(
                       time('bar.foo.bad',10),
                       time('foo.bar.baz',10)
               ),
               '45s'
       ))`, []*series{
		{
			Timestamps: []int64{120000, 165000},
			Values:     []float64{292.5, 500},
			Name:       `weightedAverage(summarize(bar.foo.bad,'45s','sum'),summarize(foo.bar.baz,'45s','sum'),summarize(bar.foo.bad,'45s','sum'),summarize(foo.bar.baz,'45s','sum'),)`,
			Tags:       map[string]string{"name": "weightedAverage(summarize(bar.foo.bad,'45s','sum'),summarize(foo.bar.baz,'45s','sum'),summarize(bar.foo.bad,'45s','sum'),summarize(foo.bar.baz,'45s','sum'),)"},
		},
	})
	f(`transformNull(
    time('foo.bar.baz',30),
    -1,
    summarize(
               group(
                       time('foo.bar.baz',10)
               ),
               '30s'
       ))`, []*series{
		{
			Timestamps:     []int64{120000, 150000, 180000},
			Values:         []float64{120, 150, 180},
			Name:           `transformNull(foo.bar.baz,-1,referenceSeries)`,
			Tags:           map[string]string{"name": "foo.bar.baz", "referenceSeries": "1", "transformNull": "-1"},
			pathExpression: ("transformNull(foo.bar.baz,-1,referenceSeries)"),
		},
	})
}

func TestExecExprFailure(t *testing.T) {
	f := func(query string) {
		t.Helper()
		ec := &evalConfig{
			startTime:   120e3,
			endTime:     420e3,
			storageStep: 60e3,
		}
		nextSeries, err := execExpr(ec, query)
		if err == nil {
			if _, err = drainAllSeries(nextSeries); err == nil {
				t.Fatalf("expecting non-nil error for query %q", query)
			}
			nextSeries = nil
		}
		if nextSeries != nil {
			t.Fatalf("expecting nil nextSeries")
		}
	}
	f("123")
	f("nonExistingFunc()")

	f("absolute()")
	f("absolute(1)")
	f("absolute('foo')")

	f("add()")
	f("add(foo.bar)")
	f("add(1.23)")
	f("add(1.23, 4.56)")
	f("add(time('a'), baz)")

	f("aggregate()")
	f("x.y|aggregate()")
	f("aggregate(1)")
	f("aggregate(time('a'), 123)")
	f("aggregate(time('a'), 123, bar.baz)")
	f("aggregate(time('a'), bar)")
	f("aggregate(time('a'), 'non-existing-func')")
	f("aggregate(time('a'), 'sum', 'bar')")
	f("aggregate(1,'sum')")

	f("aggregateLine()")
	f("aggregateLine(123)")
	f("aggregateLine(time('a'), bar)")
	f("aggregateLine(time('a'),'non-existing-func')")
	f("aggregateLine(time('a'),keepStep=aaa)")
	f("aggregateLine(time('a'),'sum',123)")

	f("aggregateWithWildcards()")
	f("aggregateWithWildcards(time('a'),bar)")
	f("aggregateWithWildcards(constantLine(123),'non-existing-func')")
	f("aggregateWithWildcards(time('a'),'sum',bar)")
	f("aggregateWithWildcards(1,'sum')")

	f("alias()")
	f("alias(time('a'))")
	f("alias(time('a'),123)")
	f("alias(1,'aa')")

	f("aliasByMetric()")
	f("aliasByMetric(123)")

	f("aliasByNode()")
	f("aliasByNode(123)")
	f("aliasByNode(time('a'),bar)")

	f("aliasByTags()")
	f("aliasByTags(123)")
	f("aliasByTags(time('a'),bar)")

	f("aliasQuery()")
	f("aliasQuery(1,2,3,4)")
	f("aliasQuery(1,'foo[bar',3,4)")
	f("aliasQuery(1,'foo',3,4)")
	f("aliasQuery(1,'foo','bar',4)")
	f("aliasQuery(constantLine(1)|alias('x'),'x','abc(de','aa')")
	f("aliasQuery(constantLine(1)|alias('x'),'x','group()','aa')")
	f("aliasQuery(1,'foo','bar','aaa')")

	f("aliasSub()")
	f("aliasSub(1,2,3)")
	f("aliasSub(1,'foo[bar',3)")
	f("aliasSub(1,'foo',3)")
	f("aliasSub(1,'foo','bar')")

	f("alpha()")
	f("alpha(1,2)")
	f("alpha(1,foo)")

	f("applyByNode()")
	f("applyByNode(1,2,3)")
	f("applyByNode(1,foo,3)")
	f("applyByNode(1,2,'aaa',4)")
	f("applyByNode(1,2,'foo')")

	f("areaBetween()")
	f("areaBetween(1)")
	f("areaBetween(group(time('1'),time('2'),time('3')))")

	f("asPercent()")
	f("asPercent(1)")
	f("asPercent(time('abc'),'foo')")
	f("asPercent(1,'foo',bar)")
	f("asPercent(time('abc'),100,1)")
	f("asPercent(time('a'),group(time('b'),time('c')))")

	f("averageAbove()")
	f("averageAbove(1,2)")
	f("averageAbove(1,foo)")

	f("averageBelow()")
	f("averageBelow(1,2)")
	f("averageBelow(1,foo)")

	f("averageOutsidePercentile()")
	f("averageOutsidePercentile(1,2)")
	f("averageOutsidePercentile(1,'foo')")

	f("averageSeries(1)")
	f("averageSeries(time('a'),1)")

	f("averageSeriesWithWildcards()")
	f("averageSeriesWithWildcards(1)")
	f("averageSeriesWithWildcards(time('a'),'foo')")

	f("avg(1)")

	f("changed()")
	f("changed(1)")

	f("color()")
	f("color(1,'foo')")
	f("color(1,foo)")

	f("consolidateBy()")
	f("consolidateBy(1,2)")
	f("consolidateBy(1,'foobar')")
	f("consolidateBy(1,'sum')")

	f("constantLine()")
	f("constantLine('foobar')")
	f("constantLine(time('a'))")
	f("constantLine(true)")
	f("constantLine(None)")
	f("constantLine(constantLine(123))")
	f("constantLine(foo=123)")
	f("constantLine(123, 456)")

	f("countSeries(1)")
	f("countSeries(time('a'),1)")

	f("cumulative()")
	f("cumulative(1)")

	f("currentAbove()")
	f("currentAbove(1,2)")
	f("currentAbove(1,foo)")

	f("currentBelow()")
	f("currentBelow(1,2)")
	f("currentBelow(1,foo)")

	f("dashed()")
	f("dashed(1)")
	f("dashed(time('a'),'foo')")

	f("delay()")
	f("delay(1,2)")
	f("delay(time('a'),'foo')")

	f("derivative()")
	f("derivative(1)")

	f("diffSeries(1)")
	f("diffSeries(time('a'),1)")

	f("divideSeries()")
	f("divideSeries(1,2)")
	f("divideSeries(time('a'),group(time('a'),time('b')))")

	f("divideSeriesLists()")
	f("divideSeriesLists(1,2)")
	f("divideSeriesLists(time('a'),2)")
	f("divideSeriesLists(time('a'),group(time('b'),time('c')))")

	f("drawAsInfinite()")
	f("drawAsInfinite(1)")

	f("events(1)")

	f("exclude()")
	f("exclude(1)")
	f("exclude(time('a'),2)")
	f("exclude(1,'foo')")
	f("exclude(1,'f[')")

	f("exp()")
	f("exp(1)")

	f("exponentialMovingAverage()")
	f("exponentialMovingAverage(1,time('a'))")
	f("exponentialMovingAverage(time('a'),'foobar')")

	f("fallbackSeries()")
	f("fallbackSeries(1,2)")
	f("fallbackSeries(group(),2)")

	f("filterSeries()")
	f("filterSeries(1,2,3,4)")
	f("filterSeries(time('a'),'foo','bar','baz')")
	f("filterSeries(time('a'),'sum',1,'baz')")
	f("filterSeries(time('a'),'sum','bar','baz')")
	f("filterSeries(time('a'),'sum','>','baz')")
	f("filterSeries(time('a'),'foo','>',3)")
	f("filterSeries(time('a'),'sum','xxx',3)")

	f("grep()")
	f("grep(1)")
	f("grep(time('a'),2)")
	f("grep(1,'foo')")
	f("grep(1,'f[')")

	f("group(1)")
	f("group('a.b.c')")
	f("group(xx=aa.bb)")
	f("group(constantLine(1),123)")

	f("groupByNode()")
	f("groupByNode(1,123)")
	f("groupByNode(1,time('a'))")
	f("groupByNode(time('a'),1,'foobar')")
	f("groupByNode(time('a'),1,2)")

	f("groupByNodes()")
	f("groupByNodes(time('a'),123)")
	f("groupByNodes(time('a'),'foobar')")
	f("groupByNodes(time('a'),'sum',time('b'))")
	f("groupByNodes(1,'sum')")

	f("groupByTags()")
	f("groupByTags(1,1)")
	f("groupByTags(1,'foo')")
	f("groupByTags(1,'sum',1)")

	f("highest()")
	f("highest(1,'foo')")
	f("highest(1,2,3)")
	f("highest(1,2,'foo')")
	f("highest(1,2,'sum')")

	f("highestAverage()")
	f("highestAverage(1,2)")
	f("highestAverage(1,'foo')")

	f("highestCurrent()")
	f("highestCurrent(1,2)")
	f("highestCurrent(1,'foo')")

	f("highestMax()")
	f("highestMax(1,2)")
	f("highestMax(1,'foo')")

	f("hitcount()")
	f("hitcount(1,2)")
	f("hitcount(1,'5min')")
	f("hitcount(1,'5min','foo')")

	f("identity()")
	f("identity(1)")

	f("integral()")
	f("integral('a')")

	f("integralByInterval()")
	f("integralByInterval(1,2)")
	f("integralByInterval(1,'1h')")

	f("interpolate()")
	f("interpolate(1)")

	f("invert()")
	f("invert(1)")

	f("keepLastValue()")
	f("keepLastValue(1)")

	f("limit()")
	f("limit(1,2)")
	f("limit(1,'foo')")

	f("lineWidth()")
	f("lineWidth(1,2)")
	f("lineWidth(1,'foo')")

	f("logarithm()")
	f("logarithm(1)")
	f("logarithm(1,'foo')")

	f("logit()")
	f("logit(1)")

	f("lowest()")
	f("lowest(1,'foo')")
	f("lowest(1,2,3)")
	f("lowest(1,2,'foo')")
	f("lowest(1,2,'sum')")

	f("lowestAverage()")
	f("lowestAverage(1,2)")
	f("lowestAverage(1,'foo')")

	f("lowestCurrent()")
	f("lowestCurrent(1,2)")
	f("lowestCurrent(1,'foo')")

	f("maxSeries(1)")
	f("maxSeries(time('a'),1)")

	f("maximumAbove()")
	f("maximumAbove(1,2)")
	f("maximumAbove(1,foo)")

	f("maximumBelow()")
	f("maximumBelow(1,2)")
	f("maximumBelow(1,foo)")

	f("minMax()")
	f("minMax(1)")

	f("minSeries(1)")
	f("minSeries(time('a'),1)")

	f("minimumAbove()")
	f("minimumAbove(1,2)")
	f("minimumAbove(1,foo)")

	f("minimumBelow()")
	f("minimumBelow(1,2)")
	f("minimumBelow(1,foo)")

	f("mostDeviant()")
	f("mostDeviant(1,2)")
	f("mostDeviant(1,foo)")

	f("movingAverage()")
	f("movingAverage(time('a'),time('b'))")
	f("movingAverage(1,1)")
	f("movingAverage(time('a),1,'foo')")
	f("movingAverage(foo=a,bar=2,baz=3)")

	f("movingMax()")
	f("movingMax(1,'5min')")
	f("movingMax(1,foo=true)")

	f("movingMedian()")
	f("movingMedian(1,'5min')")
	f("movingMedian(1,foo=true)")

	f("movingMin()")
	f("movingMin(1,'5min')")
	f("movingMin(1,foo=true)")

	f("movingSum()")
	f("movingSum(1,'5min')")
	f("movingSum(1,foo=true)")

	f("movingWindow()")
	f("movingWindow(1,foo)")
	f("movingWindow(1,'foo')")
	f("movingWindow(1,-1)")
	f("movingWindow(1,2)")
	f("movingWindow(1,2,3)")
	f("movingWindow(1,2,'non-existing-aggr-func')")
	f("movingWindow(1,2,'sum',foo)")

	f("multiplySeries(1)")
	f("multiplySeries(time('a'),1)")

	f("multiplyWithWildcards()")
	f("multiplyWithWildcards(time('a'),bar)")
	f("multiplyWithWildcards(constantLine(123),'non-existing-func')")
	f("multiplyWithWildcards(time('a'),'sum',bar)")
	f("multiplyWithWildcards(1,'sum')")

	f(`nPercentile()`)
	f(`nPercentile(1,1)`)
	f(`nPercentile(1,'foo')`)

	f(`nonNegativeDerivative()`)
	f(`nonNegativeDerivative(1)`)

	f("offset()")
	f("offset(1,2)")
	f("offset(time('a'),'fo')")

	f("offsetToZero()")
	f("offsetToZero(1)")

	f("pow()")
	f("pow(1,2)")
	f("pow(1,'foo')")

	f("rangeOfSeries(1)")
	f("rangeOfSeries(time('a'),1)")

	f("randomWalk()")
	f("randomWalk(1)")
	f("randomWalk('foo','bar')")

	f("removeAbovePercentile()")
	f("removeAbovePercentile(1, 2)")
	f("removeAbovePercentile(1, 'foo')")

	f("removeAboveValue()")
	f("removeAboveValue(1, 2)")
	f("removeAboveValue(1, 'foo')")

	f("removeBelowPercentile()")
	f("removeBelowPercentile(1, 2)")
	f("removeBelowPercentile(1, 'foo')")

	f("removeBelowValue()")
	f("removeBelowValue(1, 2)")
	f("removeBelowValue(1, 'foo')")

	f("removeBetweenPercentile()")
	f("removeBetweenPercentile(1,2)")
	f("removeBetweenPercentile(1,'foo')")

	f("removeEmptySeries()")
	f("removeEmptySeries(1)")
	f("removeEmptySeries(1,'fii')")

	f("round()")
	f("round(1)")
	f("round(1,'foo')")

	f("scale()")
	f("scale(1,2)")
	f("scale(time('a'),'foo')")

	f("setXFilesFactor()")
	f("setXFilesFactor(1,'foo')")
	f("setXFilesFactor(1,0.5)")

	f("sumWithWildcards()")
	f("sumWithWildcards(time('a'),bar)")
	f("sumWithWildcards(constantLine(123),'non-existing-func')")
	f("sumWithWildcards(time('a'),'sum',bar)")
	f("sumWithWildcards(1,'sum')")

	f("summarize()")
	f("summarize(1,2)")
	f("summarize(1,'foobar')")
	f("summarize(1,'-2min')")
	f("summarize(1, '0seconds')")
	f("summarize(1,'2min',3)")
	f("summarize(1,'1s','non-existing-func')")
	f("summarize(1,'1s','sum',3)")
	f("summarize(1,'1s')")

	f("time()")
	f("time(1)")

	f("timeFunction()")
	f("timeFunction(1)")
	f("timeFunction(False)")
	f("timeFunction(None)")
	f("timeFunction(constantLine(123))")
	f("timeFunction(foo='bar')")
	f("timeFunction('foo', 'bar')")

	f(`verticalLine("12:3420131108","event","blue",5)`)
	f(`verticalLine(10)`)
	f(`verticalLine("12:3420131108",4,5)`)
	f(`verticalLine("12:3420131108","event",4)`)
	f(`verticalLine("12:3420131108SF1bad","event")`)
	f(`verticalLine("00:01 19700101","event")`)

	f(`useSeriesAbove()`)
	f(`useSeriesAbove(1,10)`)
	f(`useSeriesAbove(1,10,10,5)`)
	f(`useSeriesAbove(1,"10",10,15)`)
	f(`useSeriesAbove(1,10,"(?=<bad>10)",15)`)
	f(`useSeriesAbove(1,10,"10",5)`)

	f(`unique(5,2)`)

	f(`perSecond()`)
	f(`perSecond(1)`)

	f(`percentileOfSeries()`)
	f(`percentileOfSeries(1)`)

	f(`substr()`)
	f(`substr(time('a'),'foo')`)
	f(`substr(time('a'),1,'foo')`)

	f(`sumSeries(1)`)
	f("sumSeries(time('a'),1)")

	f(`threshold()`)
	f(`threshold("bad arg")`)
	f(`threshold(1.5,5,"black")`)
	f(`threshold(1.5,"max",5)`)

	f(`timeShift()`)
	f(`timeShift(time('a'),1)`)
	f(`timeShift(time('a'),'foo')`)

	f(`timeSlice()`)
	f(`timeSlice(time('a'),1)`)
	f(`timeSlice(time('a'),'foo')`)
	f(`timeSlice(time('a'),'5min',1)`)
	f(`timeSlice(time('a'),'5min','bar')`)
	f(`timeSlice(1,'5min','10min')`)

	f(`timeStack()`)
	f(`timeStack(time('a'),timeShiftUnit=123)`)
	f(`timeStack(time('a'),'foo')`)
	f(`timeStack(time('a'),'1m',timeShiftStart='foo')`)
	f(`timeStack(time('a'),'1m',timeShiftEnd='bar')`)
	f(`timeStack(time('a'),'1m',10,1)`)

	f(`transformNull()`)
	f(`transformNull(1,-1,5,2)`)
	f(`transformNull(None)`)
	f(`transformNull(time('a'),2,'xxx')`)
	f(`transformNull(time('a'),'foo')`)

	f("weightedAverage()")
	f("weightedAverage(1,2)")
	f("weightedAverage(time('a'),2)")
	f("weightedAverage(time('a'),time('b'),foo.bar)")
	f("weightedAverage(time('a'),group(time('b'),time('c')))")

	f("xFilesFactor()")
	f("xFilesFactor(1,'foo')")
	f("xFilesFactor(1,0.5)")

	f(`stdev()`)
	f(`stdev(1,3,0.5)`)
	f(`stdev(1,"5",0.5)`)
	f(`stdev(1,3,"0.5")`)

	f(`stddevSeries(5)`)
	f(`stddevSeries(1)`)

	f(`stacked()`)
	f(`stacked(1)`)
	f(`stacked(1,5)`)

	f(`squareRoot()`)
	f(`squareRoot(5)`)

	f(`sortByTotal()`)
	f(`sortByTotal(1)`)

	f(`sortBy()`)
	f(`sortBy(1)`)
	f(`sortBy(1,5)`)
	f(`sortBy(1,'bad func name')`)
	f(`sortBy(1,'sum','non bool')`)

	f(`sortByName()`)
	f(`sortByName(1)`)
	f(`sortByName(1,"bad bool")`)
	f(`sortByName(1,true,"bad bool")`)
	f(`sortByName(1,5,5,6)`)

	f(`sortByMinima()`)
	f(`sortByMinima(1)`)

	f(`sortByMaxima()`)
	f(`sortByMaxima(1)`)

	f(`smartSummarize(1)`)
	f(`smartSummarize(1,"1d")`)
	f(`smartSummarize(1,"1d","sum","1light year")`)
	f(`smartSummarize(1,1)`)
	f(`smartSummarize(1,"1d",1)`)
	f(`smartSummarize(1,"1light year")`)
	f(`smartSummarize(1,"-1d")`)
	f(`smartSummarize(1,"1d","bad func")`)
	f(`smartSummarize(1,"1d","sum",true)`)

	f(`sinFunction()`)
	f(`sinFunction(5)`)
	f(`sinFunction("name","bad arg")`)
	f(`sinFunction("name",1,"bad arg")`)
	f(`sinFunction(1,-2,3)`)

	f(`sigmoid()`)
	f(`sigmoid(1)`)

	f(`scaleToSeconds()`)
	f(`scaleToSeconds(1,10)`)
	f(`scaleToSeconds(1,"10")`)

	f(`secondYAxis()`)
	f(`secondYAxis(1)`)

	f(`isNonNull()`)
	f(`isNonNull(1)`)

	f(`linearRegression()`)
	f(`linearRegression(10)`)
	f(`linearRegression(none.exist.metric)`)
	f(`linearRegression(none.exist.metric,"badarg1")`)
	f(`linearRegression(time("foo.baz",15),"-1min","badargv2")`)

	f(`holtWintersForecast()`)
	f(`holtWintersForecast(none.exist.metric)`)
	f(`holtWintersForecast(none.exist.metric,124124)`)
	f(`holtWintersForecast(none.exist.metric,7d,"ads124")`)
	f(`holtWintersForecast(none.exist.metric,"7d","ads124")`)
	f(`holtWintersForecast(none.exist.metric,"afsf","7d")`)
	f(`holtWintersForecast(none.exist.metric,"7d",124214)`)

	f(`holtWintersConfidenceBands()`)
	f(`holtWintersConfidenceBands(none.exist.metric)`)
	f(`holtWintersConfidenceBands(none.exist.metric,"124124")`)
	f(`holtWintersConfidenceBands(none.exist.metric,7,123)`)
	f(`holtWintersConfidenceBands(none.exist.metric,7,"ads124")`)
	f(`holtWintersConfidenceBands(none.exist.metric,7,"7d","ads124")`)
	f(`holtWintersConfidenceBands(none.exist.metric,7,"afsf","7d")`)
	f(`holtWintersConfidenceBands(none.exist.metric,7,"7d",124214)`)

	f(`holtWintersAberration()`)
	f(`holtWintersAberration(124)`)
	f(`holtWintersAberration(none.exist.metric)`)

	f(`holtWintersConfidenceArea(group(time("foo.baz",15),time("foo.baz",15)))`)
	f(`holtWintersConfidenceArea()`)
}

func compareSeries(ss, ssExpected []*series, expr graphiteql.Expr) error {
	if len(ss) != len(ssExpected) {
		return fmt.Errorf("unexpected series count; got %d; want %d", len(ss), len(ssExpected))
	}
	m := make(map[string]*series)
	for _, s := range ssExpected {
		m[s.Name] = s
	}
	exprStrExpected := string(expr.AppendString(nil))
	for _, s := range ss {
		sExpected := m[s.Name]
		if sExpected == nil {
			return fmt.Errorf("missing series with name %q", s.Name)
		}
		if !reflect.DeepEqual(s.Tags, sExpected.Tags) {
			return fmt.Errorf("unexpected tag for series %q\ngot\n%s\nwant\n%s", s.Name, s.Tags, sExpected.Tags)
		}
		if !reflect.DeepEqual(s.Timestamps, sExpected.Timestamps) {
			return fmt.Errorf("unexpected timestamps for series %q\ngot\n%d\nwant\n%d", s.Name, s.Timestamps, sExpected.Timestamps)
		}

		if !equalFloats(s.Values, sExpected.Values) {
			return fmt.Errorf("unexpected values for series %q\ngot\n%g\nwant\n%g", s.Name, s.Values, sExpected.Values)
		}
		expectedPathExpression := sExpected.Name
		if sExpected.pathExpression != "" {
			expectedPathExpression = sExpected.pathExpression
		}
		if expectedPathExpression != s.pathExpression {
			return fmt.Errorf("unexpected pathExpression for series %q\ngot\n%s\nwant\n%s", s.Name, s.pathExpression, expectedPathExpression)
		}
		exprStr := string(s.expr.AppendString(nil))
		if exprStr != exprStrExpected {
			return fmt.Errorf("unexpected expr for series %q\ngot\n%s\nwant\n%s", s.Name, exprStr, exprStrExpected)
		}
	}
	return nil
}

func equalFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v1 := range a {
		v2 := b[i]
		if math.IsNaN(v1) {
			if math.IsNaN(v2) {
				continue
			}
			return false
		} else if math.IsNaN(v2) {
			return false
		}
		eps := math.Abs(v1) / 1e9
		if math.Abs(v1-v2) > eps {
			return false
		}
	}
	return true
}

func printSeriess(ss []*series) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "[\n")
	for _, s := range ss {
		fmt.Fprintf(&sb, "\t{name=%q,tags=%v,timestamps=%s,values=%s}\n", s.Name, s.Tags, formatTimestamps(s.Timestamps), formatValues(s.Values))
	}
	fmt.Fprintf(&sb, "]\n")
	return sb.String()
}

func formatValues(vs []float64) string {
	if len(vs) == 0 {
		return "[]"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "[ ")
	for i, v := range vs {
		if math.IsNaN(v) {
			fmt.Fprintf(&sb, "nan")
		} else {
			fmt.Fprintf(&sb, "%g", v)
		}
		if i != len(vs)-1 {
			fmt.Fprintf(&sb, ", ")
		}
	}
	fmt.Fprintf(&sb, " ]")
	return sb.String()
}

func formatTimestamps(tss []int64) string {
	if len(tss) == 0 {
		return "[]"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "[ ")
	for i, ts := range tss {
		fmt.Fprintf(&sb, "%d", ts)
		if i != len(tss)-1 {
			fmt.Fprintf(&sb, ", ")
		}
	}
	fmt.Fprintf(&sb, " ]")
	return sb.String()
}
