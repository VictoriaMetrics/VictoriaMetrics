package analyzer

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"slices"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const FlagCheckStructPointers = "check-struct-pointers"

func NewAnalyzer() *analysis.Analyzer {
	r := &runner{}

	flags := flag.NewFlagSet("fatcontext", flag.ExitOnError)
	flags.BoolVar(&r.DetectInStructPointers, FlagCheckStructPointers, false,
		"set to true to detect potential fat contexts in struct pointers")

	return &analysis.Analyzer{
		Name:     "fatcontext",
		Doc:      "detects nested contexts in loops and function literals",
		Run:      r.run,
		Flags:    *flags,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
	}
}

var errUnknown = errors.New("unknown node type")

const (
	categoryInLoop          = "nested context in loop"
	categoryInFuncLit       = "nested context in function literal"
	categoryInStructPointer = "potential nested context in struct pointer"
	categoryUnsupported     = "unsupported nested context type"
)

type runner struct {
	DetectInStructPointers bool
}

func (r *runner) run(pass *analysis.Pass) (interface{}, error) {
	inspctr := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.ForStmt)(nil),
		(*ast.RangeStmt)(nil),
		(*ast.FuncLit)(nil),
		(*ast.FuncDecl)(nil),
	}

	inspctr.Preorder(nodeFilter, func(node ast.Node) {
		body, err := getBody(node)
		if err != nil {
			return
		}

		if body == nil {
			return
		}

		assignStmt := findNestedContext(pass, node, body.List)
		if assignStmt == nil {
			return
		}

		category := getCategory(pass, node, assignStmt)

		if r.shouldIgnoreReport(category) {
			return
		}

		fixes := r.getSuggestedFixes(pass, assignStmt, category)

		pass.Report(analysis.Diagnostic{
			Pos:            assignStmt.Pos(),
			Message:        category,
			SuggestedFixes: fixes,
		})
	})

	return nil, nil
}

func (r *runner) shouldIgnoreReport(category string) bool {
	return category == categoryInStructPointer && !r.DetectInStructPointers
}

func (r *runner) getSuggestedFixes(pass *analysis.Pass, assignStmt *ast.AssignStmt, category string) []analysis.SuggestedFix {
	switch category {
	case categoryInStructPointer, categoryUnsupported:
		return nil
	}

	suggestedStmt := ast.AssignStmt{
		Lhs:    assignStmt.Lhs,
		TokPos: assignStmt.TokPos,
		Tok:    token.DEFINE,
		Rhs:    assignStmt.Rhs,
	}
	suggested, err := render(pass.Fset, &suggestedStmt)

	var fixes []analysis.SuggestedFix
	if err == nil {
		fixes = append(fixes, analysis.SuggestedFix{
			Message: "replace `=` with `:=`",
			TextEdits: []analysis.TextEdit{
				{
					Pos:     assignStmt.Pos(),
					End:     assignStmt.End(),
					NewText: suggested,
				},
			},
		})
	}

	return fixes
}

func getCategory(pass *analysis.Pass, node ast.Node, assignStmt *ast.AssignStmt) string {
	switch node.(type) {
	case *ast.ForStmt, *ast.RangeStmt:
		return categoryInLoop
	}

	if isPointer(pass, assignStmt.Lhs[0]) {
		return categoryInStructPointer
	}

	switch node.(type) {
	case *ast.FuncLit, *ast.FuncDecl:
		return categoryInFuncLit
	default:
		return categoryUnsupported
	}
}

func getBody(node ast.Node) (*ast.BlockStmt, error) {
	switch typedNode := node.(type) {
	case *ast.ForStmt:
		return typedNode.Body, nil
	case *ast.RangeStmt:
		return typedNode.Body, nil
	case *ast.FuncLit:
		return typedNode.Body, nil
	case *ast.FuncDecl:
		return typedNode.Body, nil
	}

	return nil, errUnknown
}

func findNestedContext(pass *analysis.Pass, node ast.Node, stmts []ast.Stmt) *ast.AssignStmt {
	for _, stmt := range stmts {
		// Recurse if necessary
		switch typedStmt := stmt.(type) {
		case *ast.BlockStmt:
			if found := findNestedContext(pass, node, typedStmt.List); found != nil {
				return found
			}
		case *ast.IfStmt:
			if found := findNestedContext(pass, node, typedStmt.Body.List); found != nil {
				return found
			}
		case *ast.SwitchStmt:
			if found := findNestedContext(pass, node, typedStmt.Body.List); found != nil {
				return found
			}
		case *ast.CaseClause:
			if found := findNestedContext(pass, node, typedStmt.Body); found != nil {
				return found
			}
		case *ast.SelectStmt:
			if found := findNestedContext(pass, node, typedStmt.Body.List); found != nil {
				return found
			}
		case *ast.CommClause:
			if found := findNestedContext(pass, node, typedStmt.Body); found != nil {
				return found
			}
		}

		// Actually check for nested context
		assignStmt, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}

		t := pass.TypesInfo.TypeOf(assignStmt.Lhs[0])
		if t == nil {
			continue
		}

		if t.String() != "context.Context" {
			continue
		}

		if assignStmt.Tok == token.DEFINE {
			continue
		}

		// Ignore [context.Background] & [context.TODO].
		if isContextFunction(assignStmt.Rhs[0], "Background", "TODO") {
			continue
		}

		if isPointer(pass, assignStmt.Lhs[0]) {
			return assignStmt
		}

		// allow assignment to non-pointer children of values defined within the loop
		if isWithinLoop(assignStmt.Lhs[0], node, pass) {
			continue
		}

		return assignStmt
	}

	return nil
}

// render returns the pretty-print of the given node
func render(fset *token.FileSet, x interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, x); err != nil {
		return nil, fmt.Errorf("printing node: %w", err)
	}
	return buf.Bytes(), nil
}

func isContextFunction(exp ast.Expr, fnName ...string) bool {
	call, ok := exp.(*ast.CallExpr)
	if !ok {
		return false
	}

	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}

	return ident.Name == "context" && slices.Contains(fnName, selector.Sel.Name)
}

func isWithinLoop(exp ast.Expr, node ast.Node, pass *analysis.Pass) bool {
	lhs := getRootIdent(pass, exp)
	if lhs == nil {
		return false
	}

	obj := pass.TypesInfo.ObjectOf(lhs)
	if obj == nil {
		return false
	}

	scope := obj.Parent()
	if scope == nil {
		return false
	}

	return scope.Pos() >= node.Pos() && scope.End() <= node.End()
}

func getRootIdent(pass *analysis.Pass, node ast.Node) *ast.Ident {
	for {
		switch n := node.(type) {
		case *ast.Ident:
			return n
		case *ast.IndexExpr:
			node = n.X
		case *ast.SelectorExpr:
			if sel, ok := pass.TypesInfo.Selections[n]; ok && sel.Indirect() {
				return nil // indirected (pointer) roots don't imply a (safe) copy
			}
			node = n.X
		default:
			return nil
		}
	}
}

func isPointer(pass *analysis.Pass, exp ast.Node) bool {
	switch n := exp.(type) {
	case *ast.SelectorExpr:
		sel, ok := pass.TypesInfo.Selections[n]
		return ok && sel.Indirect()
	}

	return false
}
