package sa1006

import (
	"fmt"
	"go/ast"
	"go/types"

	"honnef.co/go/tools/analysis/code"
	"honnef.co/go/tools/analysis/edit"
	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/analysis/report"
	"honnef.co/go/tools/knowledge"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var SCAnalyzer = lint.InitializeAnalyzer(&lint.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name:     "SA1006",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	},
	Doc: &lint.RawDocumentation{
		Title: `\'Printf\' with dynamic first argument and no further arguments`,
		Text: `Using \'fmt.Printf\' with a dynamic first argument can lead to unexpected
output. The first argument is a format string, where certain character
combinations have special meaning. If, for example, a user were to
enter a string such as

    Interest rate: 5%

and you printed it with

    fmt.Printf(s)

it would lead to the following output:

    Interest rate: 5%!(NOVERB).

Similarly, forming the first parameter via string concatenation with
user input should be avoided for the same reason. When printing user
input, either use a variant of \'fmt.Print\', or use the \'%s\' Printf verb
and pass the string as an argument.`,
		Since:    "2017.1",
		Severity: lint.SeverityWarning,
		MergeIf:  lint.MergeIfAny,
	},
})

var Analyzer = SCAnalyzer.Analyzer

func run(pass *analysis.Pass) (interface{}, error) {
	fn := func(node ast.Node) {
		call := node.(*ast.CallExpr)
		name := code.CallName(pass, call)
		var arg int

		switch name {
		case "fmt.Errorf", "fmt.Printf", "fmt.Sprintf",
			"log.Fatalf", "log.Panicf", "log.Printf", "(*log.Logger).Printf",
			"(*testing.common).Logf", "(*testing.common).Errorf",
			"(*testing.common).Fatalf", "(*testing.common).Skipf",
			"(testing.TB).Logf", "(testing.TB).Errorf",
			"(testing.TB).Fatalf", "(testing.TB).Skipf":
			arg = knowledge.Arg("fmt.Printf.format")
		case "fmt.Fprintf":
			arg = knowledge.Arg("fmt.Fprintf.format")
		default:
			return
		}
		if len(call.Args) != arg+1 {
			// This filters out calls of method expressions like (*log.Logger).Printf(nil, s)
			return
		}
		switch call.Args[arg].(type) {
		case *ast.CallExpr, *ast.Ident:
		default:
			return
		}

		if _, ok := pass.TypesInfo.TypeOf(call.Args[arg]).(*types.Tuple); ok {
			// the called function returns multiple values and got
			// splatted into the call. for all we know, it is
			// returning good arguments.
			return
		}

		var alt string
		if name == "fmt.Errorf" {
			// The alternative to fmt.Errorf isn't fmt.Error but errors.New
			alt = "errors.New"
		} else {
			// This can be either a function call like log.Printf or a method call with an
			// arbitrarily complex selector, such as foo.bar[0].Printf. In either case,
			// all we have to do is remove the final 'f' from the existing call.Fun
			// expression.
			alt = report.Render(pass, call.Fun)
			alt = alt[:len(alt)-1]
		}
		report.Report(pass, call,
			"printf-style function with dynamic format string and no further arguments should use print-style function instead",
			report.Fixes(edit.Fix(fmt.Sprintf("use %s instead of %s", alt, name), edit.ReplaceWithString(call.Fun, alt))))
	}
	code.Preorder(pass, fn, (*ast.CallExpr)(nil))
	return nil, nil
}
