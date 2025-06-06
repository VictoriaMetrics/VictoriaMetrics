package st1017

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "ST1017",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Don't use Yoda conditions`,
		Text: `Yoda conditions are conditions of the kind \"if 42 == x\", where the
literal is on the left side of the comparison. These are a common
idiom in languages in which assignment is an expression, to avoid bugs
of the kind \"if (x = 42)\". In Go, which doesn't allow for this kind of
bug, we prefer the more idiomatic \"if x == 42\".`,
		Since:   "2019.2",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkYodaConditionsQ = pattern.MustParse(`(BinaryExpr left@(TrulyConstantExpression _) tok@(Or "==" "!=") right@(Not (TrulyConstantExpression _)))`)
	checkYodaConditionsR = pattern.MustParse(`(BinaryExpr right tok left)`)
)

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		if _, edits, ok := code.MatchAndEdit(pass, checkYodaConditionsQ, checkYodaConditionsR, node); ok {
			report.Report(pass, node, "don't use Yoda conditions",
				report.FilterGenerated(),
				report.Fixes(edit.Fix("un-Yoda-fy", edits...)))
		}
	}
	code.Preorder(pass, fn, (*ast.BinaryExpr)(nil))
	return nil, nil
}
