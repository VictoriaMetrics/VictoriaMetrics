package analyzer

import (
	"go/ast"
	"go/token"
	"go/types"
	"unicode"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// New returns new errname analyzer.
func New() *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     "errname",
		Doc:      "Checks that sentinel errors are prefixed with the `Err` and error types are suffixed with the `Error`.",
		Run:      run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

func run(pass *analysis.Pass) (interface{}, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.Nodes([]ast.Node{
		(*ast.TypeSpec)(nil),
		(*ast.ValueSpec)(nil),
		(*ast.FuncDecl)(nil),
	}, func(node ast.Node, push bool) bool {
		if !push {
			return false
		}

		switch v := node.(type) {
		case *ast.FuncDecl:
			return false

		case *ast.ValueSpec:
			if len(v.Names) != 1 {
				return false
			}
			ident := v.Names[0]

			if exprImplementsError(pass, ident) && !isValidErrorVarName(ident.Name) {
				reportAboutSentinelError(pass, v.Pos(), ident.Name)
			}
			return false

		case *ast.TypeSpec:
			tt := pass.TypesInfo.TypeOf(v.Name)
			if tt == nil {
				return false
			}
			// NOTE(a.telyshev): Pointer is the hack against Error() method with pointer receiver.
			if !typeImplementsError(types.NewPointer(tt)) {
				return false
			}

			name := v.Name.Name
			if _, ok := v.Type.(*ast.ArrayType); ok {
				if !isValidErrorArrayTypeName(name) {
					reportAboutArrayErrorType(pass, v.Pos(), name)
				}
			} else if !isValidErrorTypeName(name) {
				reportAboutErrorType(pass, v.Pos(), name)
			}
			return false
		}

		return true
	})

	return nil, nil //nolint:nilnil
}

func reportAboutErrorType(pass *analysis.Pass, typePos token.Pos, typeName string) {
	var form string
	if unicode.IsLower([]rune(typeName)[0]) {
		form = "xxxError"
	} else {
		form = "XxxError"
	}

	pass.Reportf(typePos, "the error type name `%s` should conform to the `%s` format", typeName, form)
}

func reportAboutArrayErrorType(pass *analysis.Pass, typePos token.Pos, typeName string) {
	var forms string
	if unicode.IsLower([]rune(typeName)[0]) {
		forms = "`xxxErrors` or `xxxError`"
	} else {
		forms = "`XxxErrors` or `XxxError`"
	}

	pass.Reportf(typePos, "the error type name `%s` should conform to the %s format", typeName, forms)
}

func reportAboutSentinelError(pass *analysis.Pass, pos token.Pos, varName string) {
	var form string
	if unicode.IsLower([]rune(varName)[0]) {
		form = "errXxx"
	} else {
		form = "ErrXxx"
	}
	pass.Reportf(pos, "the sentinel error name `%s` should conform to the `%s` format", varName, form)
}
