package tenv

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

const doc = "tenv is analyzer that detects using os.Setenv instead of t.Setenv since Go1.17"

// Analyzer is tenv analyzer
var Analyzer = &analysis.Analyzer{
	Name: "tenv",
	Doc:  doc,
	Run:  run,
	Requires: []*analysis.Analyzer{
		inspect.Analyzer,
	},
}

var (
	A     = "all"
	aflag bool
)

func init() {
	Analyzer.Flags.BoolVar(&aflag, A, false, "the all option will run against all method in test file")
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
		(*ast.FuncLit)(nil),
	}

	inspect.Preorder(nodeFilter, func(n ast.Node) {
		switch n := n.(type) {
		case *ast.FuncDecl:
			checkFuncDecl(pass, n, pass.Fset.File(n.Pos()).Name())
		case *ast.FuncLit:
			checkFuncLit(pass, n, pass.Fset.File(n.Pos()).Name())
		}
	})

	return nil, nil
}

func checkFuncDecl(pass *analysis.Pass, f *ast.FuncDecl, fileName string) {
	argName, ok := targetRunner(pass, f.Type.Params.List, fileName)
	if !ok {
		return
	}
	checkStmts(pass, f.Body.List, f.Name.Name, argName)
}

func checkFuncLit(pass *analysis.Pass, f *ast.FuncLit, fileName string) {
	argName, ok := targetRunner(pass, f.Type.Params.List, fileName)
	if !ok {
		return
	}
	checkStmts(pass, f.Body.List, "anonymous function", argName)
}

func checkStmts(pass *analysis.Pass, stmts []ast.Stmt, funcName, argName string) {
	for _, stmt := range stmts {
		switch stmt := stmt.(type) {
		case *ast.ExprStmt:
			checkExprStmt(pass, stmt, funcName, argName)
		case *ast.IfStmt:
			checkIfStmt(pass, stmt, funcName, argName)
		case *ast.AssignStmt:
			checkAssignStmt(pass, stmt, funcName, argName)
		case *ast.ForStmt:
			checkForStmt(pass, stmt, funcName, argName)
		}
	}
}

func checkExprStmt(pass *analysis.Pass, stmt *ast.ExprStmt, funcName, argName string) {
	callExpr, ok := stmt.X.(*ast.CallExpr)
	if !ok {
		return
	}
	checkArgs(pass, callExpr.Args, funcName, argName)
	ident, ok := callExpr.Fun.(*ast.Ident)
	if ok {
		obj := pass.TypesInfo.ObjectOf(ident)
		checkObj(pass, obj, stmt.Pos(), funcName, argName)
	}
	fun, ok := callExpr.Fun.(*ast.SelectorExpr)
	if ok {
		obj := pass.TypesInfo.ObjectOf(fun.Sel)
		checkObj(pass, obj, stmt.Pos(), funcName, argName)
	}
}

func checkArgs(pass *analysis.Pass, args []ast.Expr, funcName, argName string) {
	for _, arg := range args {
		callExpr, ok := arg.(*ast.CallExpr)
		if !ok {
			continue
		}
		ident, ok := callExpr.Fun.(*ast.Ident)
		if ok {
			obj := pass.TypesInfo.ObjectOf(ident)
			checkObj(pass, obj, arg.Pos(), funcName, argName)
		}
		fun, ok := callExpr.Fun.(*ast.SelectorExpr)
		if ok {
			obj := pass.TypesInfo.ObjectOf(fun.Sel)
			checkObj(pass, obj, arg.Pos(), funcName, argName)
		}
	}
}

func checkIfStmt(pass *analysis.Pass, stmt *ast.IfStmt, funcName, argName string) {
	assignStmt, ok := stmt.Init.(*ast.AssignStmt)
	if !ok {
		return
	}
	rhs, ok := assignStmt.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	ident, ok := rhs.Fun.(*ast.Ident)
	if ok {
		obj := pass.TypesInfo.ObjectOf(ident)
		checkObj(pass, obj, stmt.Pos(), funcName, argName)
	}
	fun, ok := rhs.Fun.(*ast.SelectorExpr)
	if ok {
		obj := pass.TypesInfo.ObjectOf(fun.Sel)
		checkObj(pass, obj, stmt.Pos(), funcName, argName)
	}
}

func checkAssignStmt(pass *analysis.Pass, stmt *ast.AssignStmt, funcName, argName string) {
	rhs, ok := stmt.Rhs[0].(*ast.CallExpr)
	if !ok {
		return
	}
	ident, ok := rhs.Fun.(*ast.Ident)
	if ok {
		obj := pass.TypesInfo.ObjectOf(ident)
		checkObj(pass, obj, stmt.Pos(), funcName, argName)
	}
	fun, ok := rhs.Fun.(*ast.SelectorExpr)
	if ok {
		obj := pass.TypesInfo.ObjectOf(fun.Sel)
		checkObj(pass, obj, stmt.Pos(), funcName, argName)
	}
}

func checkObj(pass *analysis.Pass, obj types.Object, pos token.Pos, funcName, argName string) {
	// For built-in objects, obj.Pkg() returns nil.
	var pkgPrefix string
	if pkg := obj.Pkg(); pkg != nil {
		pkgPrefix = pkg.Name() + "."
	}

	targetName := pkgPrefix + obj.Name()
	if targetName == "os.Setenv" {
		if argName == "" {
			argName = "testing"
		}
		pass.Reportf(pos, "os.Setenv() can be replaced by `%s.Setenv()` in %s", argName, funcName)
	}
}

func checkForStmt(pass *analysis.Pass, stmt *ast.ForStmt, funcName, argName string) {
	checkStmts(pass, stmt.Body.List, funcName, argName)
}

func targetRunner(pass *analysis.Pass, params []*ast.Field, fileName string) (string, bool) {
	for _, p := range params {
		switch typ := p.Type.(type) {
		case *ast.StarExpr:
			if checkStarExprTarget(pass, typ) {
				if len(p.Names) == 0 {
					return "", false
				}
				argName := p.Names[0].Name
				return argName, true
			}
		case *ast.SelectorExpr:
			if checkSelectorExprTarget(pass, typ) {
				if len(p.Names) == 0 {
					return "", false
				}
				argName := p.Names[0].Name
				return argName, true
			}
		}
	}
	if aflag && strings.HasSuffix(fileName, "_test.go") {
		return "", true
	}
	return "", false
}

func checkStarExprTarget(pass *analysis.Pass, typ *ast.StarExpr) bool {
	selector, ok := typ.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch pass.TypesInfo.TypeOf(selector).String() {
	case "testing.T", "testing.B":
		return true
	default:
		return false
	}
}

func checkSelectorExprTarget(pass *analysis.Pass, typ *ast.SelectorExpr) bool {
	return pass.TypesInfo.TypeOf(typ).String() == "testing.TB"
}
