package promql

import (
	"fmt"
	"github.com/VictoriaMetrics/metricsql"
	"testing"
)

func TestUserCase(t *testing.T) {
	mql := "1 and (0 > 1)"
	expr, err := metricsql.Parse(mql)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	fmt.Println(expr)
	e, _ := expr.(*metricsql.NumberExpr)
	fmt.Println(e.N)
}
