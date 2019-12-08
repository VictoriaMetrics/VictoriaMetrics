package promql

import (
	"fmt"
	"strings"
	"testing"
)

func TestWalk(t *testing.T) {
	e, err := testParser.ParseRawPromQL(`
	WITH (
		rf(a, b) = a + b
	)
	rf(metric1{foo="bar"}, metric2) or (sum(abs(changes(metric3))))
	`)
	if err != nil {
		t.Fatalf("unexpected error when parsing: %s", err)
	}
	var cv collectVisitor
	Walk(e, &cv)
	expected := []string{
		`*promql.WithExpr WITH (rf(a,b) = a + b) rf(metric1{foo="bar"}, metric2) or (sum(abs(changes(metric3))))`,
		`*promql.WithArgExpr rf(a,b) = a + b`,
		`*promql.BinaryOpExpr a + b`,
		`*promql.MetricTemplateExpr a`,
		`*promql.TagFilterExpr __name__="a"`,
		`*promql.StringTemplateExpr "a"`,
		`*promql.MetricTemplateExpr b`,
		`*promql.TagFilterExpr __name__="b"`,
		`*promql.StringTemplateExpr "b"`,
		`*promql.BinaryOpExpr rf(metric1{foo="bar"}, metric2) or (sum(abs(changes(metric3))))`,
		`*promql.FuncExpr rf(metric1{foo="bar"}, metric2)`,
		`*promql.MetricTemplateExpr metric1{foo="bar"}`,
		`*promql.TagFilterExpr __name__="metric1"`,
		`*promql.StringTemplateExpr "metric1"`,
		`*promql.TagFilterExpr foo="bar"`,
		`*promql.StringTemplateExpr "bar"`,
		`*promql.MetricTemplateExpr metric2`,
		`*promql.TagFilterExpr __name__="metric2"`,
		`*promql.StringTemplateExpr "metric2"`,
		`*promql.ParensExpr (sum(abs(changes(metric3))))`,
		`*promql.AggrFuncExpr sum(abs(changes(metric3)))`,
		`*promql.FuncExpr abs(changes(metric3))`,
		`*promql.FuncExpr changes(metric3)`,
		`*promql.MetricTemplateExpr metric3`,
		`*promql.TagFilterExpr __name__="metric3"`,
		`*promql.StringTemplateExpr "metric3"`,
		`*promql.ModifierExpr  ()`,
	}
	if len(cv.visited) != len(expected) {
		t.Fatal("Expected", len(expected), "elements visited, got", len(cv.visited))
	}
	for i, v := range cv.visited {
		if strings.TrimSpace(v) != expected[i] {
			t.Fatalf("Expected %s, got %s at position %v", expected[i], v, i)
		}
	}
}

type collectVisitor struct {
	visited []string
}

func (cv *collectVisitor) Visit(e Expr) Visitor {
	sb := e.AppendString(nil)
	s := fmt.Sprintf("%T %v\n", e, string(sb))
	cv.visited = append(cv.visited, s)
	return cv
}
