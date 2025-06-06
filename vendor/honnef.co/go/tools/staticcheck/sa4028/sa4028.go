package sa4028

import (
	"go/ast"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA4028",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title:    `\'x % 1\' is always zero`,
		Since:    "2022.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny, // MergeIfAny if we only flag literals, not named constants
	},
})

var Analyzer = SCAnalyzer.Analyzer

var moduloOneQ = pattern.MustParse(`(BinaryExpr _ "%" (IntegerLiteral "1"))`)

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		_, ok := code.Match(pass, moduloOneQ, node)
		if !ok {
			return
		}
		report.Report(pass, node, "x % 1 is always zero")
	}
	code.Preorder(pass, fn, (*ast.BinaryExpr)(nil))
	return nil, nil
}
