package s1007

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/facts/generated"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "S1007",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer, generated.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `Simplify regular expression by using raw string literal`,
		Text: `Raw string literals use backticks instead of quotation marks and do not support
any escape sequences. This means that the backslash can be used
freely, without the need of escaping.

Since regular expressions have their own escape sequences, raw strings
can improve their readability.`,
		Before:  `regexp.Compile("\\A(\\w+) profile: total \\d+\\n\\z")`,
		After:   "regexp.Compile(`\\A(\\w+) profile: total \\d+\\n\\z`)",
		Since:   "2017.1",
		MergeIf: lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		if !code.IsCallToAny(pass, call, "regexp.MustCompile", "regexp.Compile") {
			return
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return
		}
		lit, ok := call.Args[knowledge.Arg("regexp.Compile.expr")].(*ast.BasicLit)
		if !ok {
			// TODO(dominikh): support string concat, maybe support constants
			return
		}
		if lit.Kind != token.STRING {
			// invalid function call
			return
		}
		if lit.Value[0] != '"' {
			// already a raw string
			return
		}
		val := lit.Value
		if !strings.Contains(val, `\\`) {
			return
		}
		if strings.Contains(val, "`") {
			return
		}

		bs := false
		for _, c := range val {
			if !bs && c == '\\' {
				bs = true
				continue
			}
			if bs && c == '\\' {
				bs = false
				continue
			}
			if bs {
				// backslash followed by non-backslash -> escape sequence
				return
			}
		}

		report.Report(pass, call, fmt.Sprintf("should use raw string (`...`) with regexp.%s to avoid having to escape twice", sel.Sel.Name), report.FilterGenerated())
	}
	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
