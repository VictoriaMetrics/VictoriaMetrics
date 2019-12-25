package metricsql_test

import (
	"fmt"
	"log"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/metricsql"
)

func ExampleParse() {
	expr, err := metricsql.Parse(`sum(rate(foo{bar="baz"}[5m])) by (x,y)`)
	if err != nil {
		log.Fatalf("parse error: %s", err)
	}
	fmt.Printf("parsed expr: %s\n", expr.AppendString(nil))

	ae := expr.(*metricsql.AggrFuncExpr)
	fmt.Printf("aggr func: name=%s, arg=%s, modifier=%s\n", ae.Name, ae.Args[0].AppendString(nil), ae.Modifier.AppendString(nil))

	fe := ae.Args[0].(*metricsql.FuncExpr)
	fmt.Printf("func: name=%s, arg=%s\n", fe.Name, fe.Args[0].AppendString(nil))

	re := fe.Args[0].(*metricsql.RollupExpr)
	fmt.Printf("rollup: expr=%s, window=%s\n", re.Expr.AppendString(nil), re.Window)

	me := re.Expr.(*metricsql.MetricExpr)
	fmt.Printf("metric: labelFilter1=%s, labelFilter2=%s", me.LabelFilters[0].AppendString(nil), me.LabelFilters[1].AppendString(nil))

	// Output:
	// parsed expr: sum(rate(foo{bar="baz"}[5m])) by (x, y)
	// aggr func: name=sum, arg=rate(foo{bar="baz"}[5m]), modifier=by (x, y)
	// func: name=rate, arg=foo{bar="baz"}[5m]
	// rollup: expr=foo{bar="baz"}, window=5m
	// metric: labelFilter1=__name__="foo", labelFilter2=bar="baz"
}
