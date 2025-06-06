package sa4030

import (
	"fmt"
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
		Name:     "SA4030",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: "Ineffective attempt at generating random number",
		Text: `
Functions in the \'math/rand\' package that accept upper limits, such
as \'Intn\', generate random numbers in the half-open interval [0,n). In
other words, the generated numbers will be \'>= 0\' and \'< n\' – they
don't include \'n\'. \'rand.Intn(1)\' therefore doesn't generate \'0\'
or \'1\', it always generates \'0\'.`,
		Since:    "2022.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var ineffectiveRandIntQ = pattern.MustParse(`
	(CallExpr
		(Symbol
			name@(Or
				"math/rand.Int31n"
				"math/rand.Int63n"
				"math/rand.Intn"
				"(*math/rand.Rand).Int31n"
				"(*math/rand.Rand).Int63n"
				"(*math/rand.Rand).Intn"))
		[(IntegerLiteral "1")])`)

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		m, ok := code.Match(pass, ineffectiveRandIntQ, node)
		if !ok {
			return
		}

		report.Report(pass, node,
			fmt.Sprintf("%s(n) generates a random value 0 <= x < n; that is, the generated values don't include n; %s therefore always returns 0",
				m.State["name"], report.Render(pass, node)))
	}

	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
