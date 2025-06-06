package sa1004

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/pattern"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1004",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Suspiciously small untyped constant in \'time.Sleep\'`,
		Text: `The \'time\'.Sleep function takes a \'time.Duration\' as its only argument.
Durations are expressed in nanoseconds. Thus, calling \'time.Sleep(1)\'
will sleep for 1 nanosecond. This is a common source of bugs, as sleep
functions in other languages often accept seconds or milliseconds.

The \'time\' package provides constants such as \'time.Second\' to express
large durations. These can be combined with arithmetic to express
arbitrary durations, for example \'5 * time.Second\' for 5 seconds.

If you truly meant to sleep for a tiny amount of time, use
\'n * time.Nanosecond\' to signal to Staticcheck that you did mean to sleep
for some amount of nanoseconds.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

var (
	checkTimeSleepConstantPatternQ   = pattern.MustParse(`(CallExpr (Symbol "time.Sleep") lit@(IntegerLiteral value))`)
	checkTimeSleepConstantPatternRns = pattern.MustParse(`(BinaryExpr duration "*" (SelectorExpr (Ident "time") (Ident "Nanosecond")))`)
	checkTimeSleepConstantPatternRs  = pattern.MustParse(`(BinaryExpr duration "*" (SelectorExpr (Ident "time") (Ident "Second")))`)
)

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		m, ok := code.Match(pass, checkTimeSleepConstantPatternQ, node)
		if !ok {
			return
		}
		n, ok := constant.Int64Val(m.State["value"].(types.TypeAndValue).Value)
		if !ok {
			return
		}
		if n == 0 || n > 120 {
			// time.Sleep(0) is a seldom used pattern in concurrency
			// tests. >120 might be intentional. 120 was chosen
			// because the user could've meant 2 minutes.
			return
		}

		lit := m.State["lit"].(ast.Node)
		report.Report(pass, lit,
			fmt.Sprintf("sleeping for %d nanoseconds is probably a bug; be explicit if it isn't", n), report.Fixes(
				edit.Fix("explicitly use nanoseconds", edit.ReplaceWithPattern(pass.Fset, lit, checkTimeSleepConstantPatternRns, pattern.State{"duration": lit})),
				edit.Fix("use seconds", edit.ReplaceWithPattern(pass.Fset, lit, checkTimeSleepConstantPatternRs, pattern.State{"duration": lit}))))
	}
	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
