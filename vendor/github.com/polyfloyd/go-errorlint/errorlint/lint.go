package errorlint

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

type ByPosition []analysis.Diagnostic

func (l ByPosition) Len() int      { return len(l) }
func (l ByPosition) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

func (l ByPosition) Less(i, j int) bool {
	return l[i].Pos < l[j].Pos
}

func LintFmtErrorfCalls(fset *token.FileSet, info types.Info, multipleWraps bool) []analysis.Diagnostic {
	var lints []analysis.Diagnostic

	for expr, t := range info.Types {
		// Search for error expressions that are the result of fmt.Errorf
		// invocations.
		if t.Type.String() != "error" {
			continue
		}
		call, ok := isFmtErrorfCallExpr(info, expr)
		if !ok {
			continue
		}

		// Find all % fields in the format string.
		formatVerbs, ok := printfFormatStringVerbs(info, call)
		if !ok {
			continue
		}

		// For any arguments that are errors, check whether the wrapping verb is used. %w may occur
		// for multiple errors in one Errorf invocation, unless multipleWraps is true. We raise an
		// issue if at least one error does not have a corresponding wrapping verb.
		args := call.Args[1:]
		if !multipleWraps {
			wrapCount := 0
			for i := 0; i < len(args) && i < len(formatVerbs); i++ {
				arg := args[i]
				if !implementsError(info.Types[arg].Type) {
					continue
				}
				verb := formatVerbs[i]

				if verb.format == "w" {
					wrapCount++
					if wrapCount > 1 {
						lints = append(lints, analysis.Diagnostic{
							Message: "only one %w verb is permitted per format string",
							Pos:     arg.Pos(),
						})
						break
					}
				}

				if wrapCount == 0 {
					lints = append(lints, analysis.Diagnostic{
						Message: "non-wrapping format verb for fmt.Errorf. Use `%w` to format errors",
						Pos:     args[i].Pos(),
					})
					break
				}
			}

		} else {
			var lint *analysis.Diagnostic
			argIndex := 0
			for _, verb := range formatVerbs {
				if verb.index != -1 {
					argIndex = verb.index
				} else {
					argIndex++
				}

				if verb.format == "w" || verb.format == "T" {
					continue
				}
				if argIndex-1 >= len(args) {
					continue
				}
				arg := args[argIndex-1]
				if !implementsError(info.Types[arg].Type) {
					continue
				}

				strStart := call.Args[0].Pos()
				if lint == nil {
					lint = &analysis.Diagnostic{
						Message: "non-wrapping format verb for fmt.Errorf. Use `%w` to format errors",
						Pos:     arg.Pos(),
					}
				}
				lint.SuggestedFixes = append(lint.SuggestedFixes, analysis.SuggestedFix{
					Message: "Use `%w` to format errors",
					TextEdits: []analysis.TextEdit{{
						Pos:     strStart + token.Pos(verb.formatOffset) + 1,
						End:     strStart + token.Pos(verb.formatOffset) + 2,
						NewText: []byte("w"),
					}},
				})
			}
			if lint != nil {
				lints = append(lints, *lint)
			}
		}
	}
	return lints
}

// printfFormatStringVerbs returns a normalized list of all the verbs that are used per argument to
// the printf function. The index of each returned element corresponds to the index of the
// respective argument.
func printfFormatStringVerbs(info types.Info, call *ast.CallExpr) ([]verb, bool) {
	if len(call.Args) <= 1 {
		return nil, false
	}
	strLit, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		// Ignore format strings that are not literals.
		return nil, false
	}
	formatString := constant.StringVal(info.Types[strLit].Value)

	pp := printfParser{str: formatString}
	verbs, err := pp.ParseAllVerbs()
	if err != nil {
		return nil, false
	}

	return verbs, true
}

func isFmtErrorfCallExpr(info types.Info, expr ast.Expr) (*ast.CallExpr, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	fn, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		// TODO: Support fmt.Errorf variable aliases?
		return nil, false
	}
	obj := info.Uses[fn.Sel]

	pkg := obj.Pkg()
	if pkg != nil && pkg.Name() == "fmt" && obj.Name() == "Errorf" {
		return call, true
	}
	return nil, false
}

func LintErrorComparisons(info *TypesInfoExt) []analysis.Diagnostic {
	var lints []analysis.Diagnostic

	for expr := range info.TypesInfo.Types {
		// Find == and != operations.
		binExpr, ok := expr.(*ast.BinaryExpr)
		if !ok {
			continue
		}
		if binExpr.Op != token.EQL && binExpr.Op != token.NEQ {
			continue
		}
		// Comparing errors with nil is okay.
		if isNil(binExpr.X) || isNil(binExpr.Y) {
			continue
		}
		// Find comparisons of which one side is a of type error.
		if !isErrorType(info.TypesInfo, binExpr.X) && !isErrorType(info.TypesInfo, binExpr.Y) {
			continue
		}
		// Some errors that are returned from some functions are exempt.
		if isAllowedErrorComparison(info, binExpr.X, binExpr.Y) {
			continue
		}
		// Comparisons that happen in `func (type) Is(error) bool` are okay.
		if isNodeInErrorIsFunc(info, binExpr) {
			continue
		}

		lints = append(lints, analysis.Diagnostic{
			Message: fmt.Sprintf("comparing with %s will fail on wrapped errors. Use errors.Is to check for a specific error", binExpr.Op),
			Pos:     binExpr.Pos(),
		})
	}

	for scope := range info.TypesInfo.Scopes {
		// Find value switch blocks.
		switchStmt, ok := scope.(*ast.SwitchStmt)
		if !ok {
			continue
		}
		// Check whether the switch operates on an error type.
		if !isErrorType(info.TypesInfo, switchStmt.Tag) {
			continue
		}

		var problematicCaseClause *ast.CaseClause
	outer:
		for _, stmt := range switchStmt.Body.List {
			caseClause := stmt.(*ast.CaseClause)
			for _, caseExpr := range caseClause.List {
				if isNil(caseExpr) {
					continue
				}
				// Some errors that are returned from some functions are exempt.
				if !isAllowedErrorComparison(info, switchStmt.Tag, caseExpr) {
					problematicCaseClause = caseClause
					break outer
				}
			}
		}
		if problematicCaseClause == nil {
			continue
		}
		// Comparisons that happen in `func (type) Is(error) bool` are okay.
		if isNodeInErrorIsFunc(info, switchStmt) {
			continue
		}

		if switchComparesNonNil(switchStmt) {
			lints = append(lints, analysis.Diagnostic{
				Message: "switch on an error will fail on wrapped errors. Use errors.Is to check for specific errors",
				Pos:     problematicCaseClause.Pos(),
			})
		}
	}

	return lints
}

func isNil(ex ast.Expr) bool {
	ident, ok := ex.(*ast.Ident)
	return ok && ident.Name == "nil"
}

func isErrorType(info *types.Info, ex ast.Expr) bool {
	t := info.Types[ex].Type
	return t != nil && t.String() == "error"
}

func isNodeInErrorIsFunc(info *TypesInfoExt, node ast.Node) bool {
	funcDecl := info.ContainingFuncDecl(node)
	if funcDecl == nil {
		return false
	}

	if funcDecl.Name.Name != "Is" {
		return false
	}
	if funcDecl.Recv == nil {
		return false
	}
	// There should be 1 argument of type error.
	if ii := funcDecl.Type.Params.List; len(ii) != 1 || info.TypesInfo.Types[ii[0].Type].Type.String() != "error" {
		return false
	}
	// The return type should be bool.
	if ii := funcDecl.Type.Results.List; len(ii) != 1 || info.TypesInfo.Types[ii[0].Type].Type.String() != "bool" {
		return false
	}

	return true
}

// switchComparesNonNil returns true if one of its clauses compares by value.
func switchComparesNonNil(switchStmt *ast.SwitchStmt) bool {
	for _, caseBlock := range switchStmt.Body.List {
		caseClause, ok := caseBlock.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, clause := range caseClause.List {
			switch clause := clause.(type) {
			case nil:
				// default label is safe
				continue
			case *ast.Ident:
				// `case nil` is safe
				if clause.Name == "nil" {
					continue
				}
			}
			// anything else (including an Ident other than nil) isn't safe
			return true
		}
	}
	return false
}

func LintErrorTypeAssertions(fset *token.FileSet, info *TypesInfoExt) []analysis.Diagnostic {
	var lints []analysis.Diagnostic

	for expr := range info.TypesInfo.Types {
		// Find type assertions.
		typeAssert, ok := expr.(*ast.TypeAssertExpr)
		if !ok {
			continue
		}

		// Find type assertions that operate on values of type error.
		if !isErrorTypeAssertion(*info.TypesInfo, typeAssert) {
			continue
		}

		if isNodeInErrorIsFunc(info, typeAssert) {
			continue
		}

		// If the asserted type is not an error, allow the expression.
		if !implementsError(info.TypesInfo.Types[typeAssert.Type].Type) {
			continue
		}

		lints = append(lints, analysis.Diagnostic{
			Message: "type assertion on error will fail on wrapped errors. Use errors.As to check for specific errors",
			Pos:     typeAssert.Pos(),
		})
	}

	for scope := range info.TypesInfo.Scopes {
		// Find type switches.
		typeSwitch, ok := scope.(*ast.TypeSwitchStmt)
		if !ok {
			continue
		}

		// Find the type assertion in the type switch.
		var typeAssert *ast.TypeAssertExpr
		switch t := typeSwitch.Assign.(type) {
		case *ast.ExprStmt:
			typeAssert = t.X.(*ast.TypeAssertExpr)
		case *ast.AssignStmt:
			typeAssert = t.Rhs[0].(*ast.TypeAssertExpr)
		}

		// Check whether the type switch is on a value of type error.
		if !isErrorTypeAssertion(*info.TypesInfo, typeAssert) {
			continue
		}

		if isNodeInErrorIsFunc(info, typeSwitch) {
			continue
		}

		lints = append(lints, analysis.Diagnostic{
			Message: "type switch on error will fail on wrapped errors. Use errors.As to check for specific errors",
			Pos:     typeAssert.Pos(),
		})
	}

	return lints
}

func isErrorTypeAssertion(info types.Info, typeAssert *ast.TypeAssertExpr) bool {
	t := info.Types[typeAssert.X]
	return t.Type.String() == "error"
}

func implementsError(t types.Type) bool {
	mset := types.NewMethodSet(t)

	for i := 0; i < mset.Len(); i++ {
		if mset.At(i).Kind() != types.MethodVal {
			continue
		}

		obj := mset.At(i).Obj()
		if obj.Name() == "Error" && obj.Type().String() == "func() string" {
			return true
		}
	}

	return false
}
